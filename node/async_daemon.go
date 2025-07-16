package node 

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"utxo_cost/chain"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"encoding/base64"
)

type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type RPCResponse struct {
	Result json.RawMessage `json:"result"`
	Error  interface{}     `json:"error"`
	ID     int             `json:"id"`
}

type BTCDaemon struct {
	url     string
	auth    string
	idCount int32
	clients []*http.Client
}

type BlockFetcher struct {
	daemon      *BTCDaemon
	startHeight int
	out         chan *chain.Block
	concurrency int
}

func NewBTCDaemon(url, user, pass string, sessionCount int) *BTCDaemon {
	clients := make([]*http.Client, sessionCount)
	for i := 0; i < sessionCount; i++ {
		clients[i] = &http.Client{}
	}
	return &BTCDaemon{
		url:     url,
		auth:    "Basic " + basicAuth(user, pass),
		clients: clients,
	}
}

func basicAuth(username, password string) string {
	return b64Encode(fmt.Sprintf("%s:%s", username, password))
}

func b64Encode(str string) string {
    return base64.StdEncoding.EncodeToString([]byte(str))
}

func (d *BTCDaemon) Call(method string, params []interface{}) ([]byte, error) {
	id := atomic.AddInt32(&d.idCount, 1)
	reqData := RPCRequest{
		JSONRPC: "1.0",
		ID:      int(id),
		Method:  method,
		Params:  params,
	}

	payload, err := json.Marshal(reqData)
	if err != nil {
		return nil, err
	}

	client := d.clients[id%int32(len(d.clients))]
	req, err := http.NewRequest("POST", d.url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", d.auth)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	return ioutil.ReadAll(resp.Body)
}



func NewBlockFetcher(d *BTCDaemon, startHeight int, out chan *chain.Block, concurrency int) *BlockFetcher {
	return &BlockFetcher{
		daemon:      d,
		startHeight: startHeight,
		out:         out,
		concurrency: concurrency,
	}
}

func (bf *BlockFetcher) Run(ctx context.Context) {
	var wg sync.WaitGroup
	tasks := make(chan int, bf.concurrency)
	pending := make(map[int]*chain.Block)
	var mu sync.Mutex
	cond := sync.NewCond(&mu)
	nextHeight := bf.startHeight

	// Worker goroutines
	for i := 0; i < bf.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for height := range tasks {
				block := bf.fetchBlock(height)
				if block != nil {
					mu.Lock()
					pending[height] = block
					cond.Signal() // notify waiter
					mu.Unlock()
				}
			}
		}()
	}

	// Dispatcher: ensures ordered delivery to bf.out
	go func() {
		for {
			mu.Lock()
			for {
				// Only proceed if next block is ready or context is done
				if ctx.Err() != nil {
					mu.Unlock()
					return
				}
				if blk, ok := pending[nextHeight]; ok {
					delete(pending, nextHeight)
					bf.out <- blk
					nextHeight++
				} else {
					break // wait until the next needed block is ready
				}
			}
			cond.Wait()
			mu.Unlock()
		}
	}()

	// Feed heights to workers
	go func() {
		height := bf.startHeight
		for {
			select {
			case <-ctx.Done():
				close(tasks)
				wg.Wait()
				return
			default:
				tasks <- height
				height++
			}
		}
	}()
}


func (bf *BlockFetcher) fetchBlock(height int) *chain.Block {
	hashData, err := bf.daemon.Call("getblockhash", []interface{}{height})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fetchBlock] getblockhash failed for height %d: %v\n", height, err)
		return nil
	}
	var hashResp RPCResponse
	if err := json.Unmarshal(hashData, &hashResp); err != nil {
		fmt.Fprintf(os.Stderr, "[fetchBlock] hash unmarshal failed: %v\n", err)
		return nil
	}

	var hash string
	if err := json.Unmarshal(hashResp.Result, &hash); err != nil {
		fmt.Fprintf(os.Stderr, "[fetchBlock] string unmarshal failed: %v\n", err)
		return nil
	}

	blockData, err := bf.daemon.Call("getblock", []interface{}{hash, 2})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fetchBlock] getblock failed for hash %s: %v\n", hash, err)
		return nil
	}

	var blockResp RPCResponse
	if err := json.Unmarshal(blockData, &blockResp); err != nil {
		fmt.Fprintf(os.Stderr, "[fetchBlock] response unmarshal failed: %v\n", err)
		return nil
	}

	var block chain.Block
	if err := json.Unmarshal(blockResp.Result, &block); err != nil {
		fmt.Fprintf(os.Stderr, "[fetchBlock] block unmarshal failed: %v\n", err)
		return nil
	}

	return &block
}



func main() {
	daemon := NewBTCDaemon("http://127.0.0.1:8332", "rpcuser", "rpcpassword", 15)
	blockChan := make(chan *chain.Block, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fetcher := NewBlockFetcher(daemon, 800000, blockChan, 15)
	go fetcher.Run(ctx)

	// Example: receive 10 blocks
	for i := 0; i < 10; i++ {
		block := <-blockChan
		fmt.Printf("Received block %d: %s...\n", i, string(block.Hash))
	}
}
