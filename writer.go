package main

import (
	"sync"
	"time"

	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"
)

func StartUsedUTXOWriterParquet(filePath string, input <-chan UsedUTXO, flushSize int, flushInterval time.Duration, wg *sync.WaitGroup) {
	fw, err := local.NewLocalFileWriter(filePath)
	if err != nil {
		panic("failed to open Parquet file: " + err.Error())
	}

	pw, err := writer.NewParquetWriter(fw, new(UsedUTXO), 4)
	if err != nil {
		panic("failed to create Parquet writer: " + err.Error())
	}
	pw.CompressionType = parquet.CompressionCodec_SNAPPY

	buffer := make([]UsedUTXO, 0, flushSize)
	ticker := time.NewTicker(flushInterval)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer ticker.Stop()
		defer func() {
			for _, item := range buffer {
				_ = pw.Write(item)
			}
			_ = pw.WriteStop()
			_ = fw.Close()
		}()

		for {
			select {
			case utxo, ok := <-input:
				if !ok {
					return // graceful exit on channel close
				}
				buffer = append(buffer, utxo)
				if len(buffer) >= flushSize {
					for _, item := range buffer {
						_ = pw.Write(item)
					}
					buffer = buffer[:0]
				}
			case <-ticker.C:
				if len(buffer) > 0 {
					for _, item := range buffer {
						_ = pw.Write(item)
					}
					buffer = buffer[:0]
				}
			}
		}
	}()
}

