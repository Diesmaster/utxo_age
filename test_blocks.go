package main

import (
	"context"
	"fmt"
	"utxo_cost/node"
	"utxo_cost/config"// <-- this is the folder name
	"utxo_cost/chain"// <-- this is the folder name
)

func main() {
	config.PrintConfig()

	daemon := node.NewBTCDaemon(
		config.RPCURL,
		config.RPCUser,
		config.RPCPassword,
		15,
	)
	blockChan := make(chan *chain.Block, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fetcher := node.NewBlockFetcher(daemon, 180500, blockChan, 15)
	go fetcher.Run(ctx)

	for i := 0; i < 1; i++ {
		block := <-blockChan
		fmt.Printf("Received block at height %d: %s\n", block.Height, block.Hash)

		for j, tx := range block.Tx {
			fmt.Printf("  Tx %d:\n", j)
			fmt.Printf("    TxID: %s\n", tx.TxID)
			fmt.Printf("    Inputs (%d):\n", len(tx.Vin))
			for k, vin := range tx.Vin {
				fmt.Printf("      Vin %d: txid=%s, vout=%d, coinbase=%s\n", k, vin.TxID, vin.Vout, vin.Coinbase)
			}
			fmt.Printf("    Outputs (%d):\n", len(tx.Vout))
			for k, vout := range tx.Vout {
				fmt.Printf("      Vout %d: value=%.8f BTC, type=%s, addresses=%v\n",
					k, vout.Value, vout.ScriptPubKey.Type, vout.ScriptPubKey.Addresses)
			}
		}
	}
}


