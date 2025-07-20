package main

import (
	"time"

	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/writer"
	"github.com/xitongsys/parquet-go/parquet"
)

func StartUsedUTXOWriterParquet(filePath string, input <-chan UsedUTXO, flushSize int, flushInterval time.Duration) {
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

	go func() {
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
					return
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

