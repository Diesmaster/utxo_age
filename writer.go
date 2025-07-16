package main

import (
	"encoding/json"
	"os"
	"time"
)

func StartUsedUTXOWriter(filePath string, input <-chan UsedUTXO, flushSize int, flushInterval time.Duration) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic("failed to open UTXO log file: " + err.Error())
	}
	encoder := json.NewEncoder(file)
	buffer := make([]UsedUTXO, 0, flushSize)
	ticker := time.NewTicker(flushInterval)

	go func() {
		defer file.Close()
		for {
			select {
			case utxo, ok := <-input:
				if !ok {
					// Channel closed, flush final data
					for _, item := range buffer {
						_ = encoder.Encode(item)
					}
					return
				}
				buffer = append(buffer, utxo)
				if len(buffer) >= flushSize {
					for _, item := range buffer {
						_ = encoder.Encode(item)
					}
					buffer = buffer[:0]
				}

			case <-ticker.C:
				if len(buffer) > 0 {
					for _, item := range buffer {
						_ = encoder.Encode(item)
					}
					buffer = buffer[:0]
				}
			}
		}
	}()
}

