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
	"time"
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

	for i := 0; i < bf.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for height := range tasks {
				bf.fetchBlock(height)
			}
		}()
	}

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
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (bf *BlockFetcher) fetchBlock(height int) {
	hashData, err := bf.daemon.Call("getblockhash", []interface{}{height})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fetchBlock] getblockhash failed for height %d: %v\n", height, err)
		return
	}
	var hashResp RPCResponse
	if err := json.Unmarshal(hashData, &hashResp); err != nil {
		fmt.Fprintf(os.Stderr, "[fetchBlock] hash unmarshal failed: %v\n", err)
		return
	}

	var hash string
	if err := json.Unmarshal(hashResp.Result, &hash); err != nil {
		fmt.Fprintf(os.Stderr, "[fetchBlock] string unmarshal failed: %v\n", err)
		return
	}

	blockData, err := bf.daemon.Call("getblock", []interface{}{hash, 2})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fetchBlock] getblock failed for hash %s: %v\n", hash, err)
		return
	}

	var blockResp RPCResponse
	if err := json.Unmarshal(blockData, &blockResp); err != nil {
		fmt.Fprintf(os.Stderr, "[fetchBlock] hash unmarshal failed: %v\n", err)
		return
	}


	var block chain.Block
	if err := json.Unmarshal(blockResp.Result, &block); err != nil {
		fmt.Fprintf(os.Stderr, "[fetchBlock] block unmarshal failed: %v\n", err)
		return
	}

	//fmt.Printf("[fetchBlock] Raw block data: %s\n", string(blockData))
	//fmt.Printf("[fetchBlock] Parsed block: %+v\n", block)

	bf.out <- &block // 🧠 Send pointer, not the whole object
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
