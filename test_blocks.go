package main

import (
	"context"
	"fmt"
	"time"
	"utxo_cost/node"
	"utxo_cost/config"// <-- this is the folder name
	"utxo_cost/chain"// <-- this is the folder name
)

type ActiveUTXO struct {
	Value        float64 // Value in BTC
	CreatedHeight int    // The block height it was created at
}

type UsedUTXO struct {
	Value        float64 // Value in BTC
	CreatedHeight int    // The block height it was created at
	UsedHeight int
}


func main() {
	config.PrintConfig()

	utxos := make(map[string]ActiveUTXO)

	daemon := node.NewBTCDaemon(
		config.RPCURL,
		config.RPCUser,
		config.RPCPassword,
		15,
	)

	usedChan := make(chan UsedUTXO, 10000)
	StartUsedUTXOWriter("used_utxos.jsonl", usedChan, 1000, 3*time.Second)
	blockChan := make(chan *chain.Block, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fetcher := node.NewBlockFetcher(daemon, 000000, blockChan, 15)
	go fetcher.Run(ctx)

	startTime := time.Now()
	printInterval := 1000

	for i := 0; i < 100000; i++ {
		block := <-blockChan

		for _, tx := range block.Tx {

			// Handle VOUTs: create new UTXOs
			for _, vout := range tx.Vout {
				utxoKey := fmt.Sprintf("%s_%d", tx.TxID, vout.N)
				utxos[utxoKey] = ActiveUTXO{
					Value:         vout.Value,
					CreatedHeight: block.Height,
				}
			}

			// Handle VINS: consume previous UTXOs
			for _, vin := range tx.Vin {
				// skip coinbase
				if vin.Coinbase != "" {
					continue
				}

				utxoKey := fmt.Sprintf("%s_%d", vin.TxID, vin.Vout)
				if utxo, ok := utxos[utxoKey]; ok {
					delete(utxos, utxoKey)
					usedChan <- UsedUTXO{
						Value:         utxo.Value,
						CreatedHeight: utxo.CreatedHeight,
						UsedHeight:    block.Height,
					}
			}
			}
		}
		if i > 0 && i%printInterval == 0 {
			elapsed := time.Since(startTime)
			avgPerBlock := elapsed / time.Duration(printInterval)
			fmt.Printf(
				"⏱️Block %d Processed %d blocks in %s (avg: %s per block)\n",
				block.Height, printInterval, elapsed.Round(time.Millisecond), avgPerBlock.Round(time.Microsecond),
			)
			startTime = time.Now() // reset for next 1000
		}
	}
	close(usedChan)
	
}




