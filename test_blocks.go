package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"utxo_cost/node"
	"utxo_cost/config" // <-- this is the folder name
	"utxo_cost/chain"  // <-- this is the folder name
)

type UsedUTXO struct {
	Value         float64 `parquet:"name=value, type=DOUBLE"`
	CreatedHeight int     `parquet:"name=created_height, type=INT32"`
	UsedHeight    int     `parquet:"name=used_height, type=INT32"`
}

func main() {
	config.PrintConfig()

	utxos := NewActiveUTXOStore(8_000_000, "./utxo_disk_db")
	defer utxos.Close()

	daemon := node.NewBTCDaemon(
		config.RPCURL,
		config.RPCUser,
		config.RPCPassword,
		15,
	)

	usedChan := make(chan UsedUTXO, 10000)
	var wg sync.WaitGroup
	StartUsedUTXOWriterParquet("used_utxos.parquet", usedChan, 1000, 3*time.Second, &wg)

	blockChan := make(chan *chain.Block, 20)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fetcher := node.NewBlockFetcher(daemon, 0, blockChan, 15)
	go fetcher.Run(ctx)

	startTime := time.Now()
	printInterval := 1000

	for i := 0; i < 900000; i++ {
		block := <-blockChan

		for _, tx := range block.Tx {
			// Handle VOUTs: create new UTXOs
			for _, vout := range tx.Vout {
				utxoKey := fmt.Sprintf("%s_%d", tx.TxID, vout.N)
				utxos.Add(utxoKey, ActiveUTXO{
					Value:         vout.Value,
					CreatedHeight: block.Height,
				})
			}

			// Handle VINS: consume previous UTXOs
			for _, vin := range tx.Vin {
				if vin.Coinbase != "" {
					continue
				}

				utxoKey := fmt.Sprintf("%s_%d", vin.TxID, vin.Vout)
				if utxo, ok := utxos.Get(utxoKey); ok {
					utxos.Delete(utxoKey)
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
				"⏱️ Block %d — Processed %d blocks in %s (avg: %s per block)\n",
				block.Height, printInterval, elapsed.Round(time.Millisecond), avgPerBlock.Round(time.Microsecond),
			)
			startTime = time.Now()
		}
	}

	close(usedChan) // important: signal writer to finish
	wg.Wait()       // wait for writer to flush and close
}

