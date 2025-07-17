package main

import (
	"encoding/json"
	"fmt"
	"github.com/dgraph-io/badger/v4"
	"log"
	"sync"
	"os"
)

type ActiveUTXOStore struct {
	mu        sync.Mutex
	mem       map[string]ActiveUTXO
	order     []string
	threshold int
	db        *badger.DB
}

type ActiveUTXO struct {
	Value        float64 // Value in BTC
	CreatedHeight int    // The block height it was created at
}


func NewActiveUTXOStore(threshold int, dbPath string) *ActiveUTXOStore {
	opts := badger.DefaultOptions(dbPath).WithLogger(nil)
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatalf("Failed to open BadgerDB: %v", err)
	}
	return &ActiveUTXOStore{
		mem:       make(map[string]ActiveUTXO),
		order:     make([]string, 0, threshold),
		threshold: threshold,
		db:        db,
	}
}

func (s *ActiveUTXOStore) Add(key string, utxo ActiveUTXO) {
	s.mu.Lock()
	defer s.mu.Unlock()

	REMOVAL_RATE := 0.90

	s.mem[key] = utxo
	s.order = append(s.order, key)

	if len(s.mem) > s.threshold {
		s.offloadOldest(REMOVAL_RATE) // remove 30%
	}
}

func (s *ActiveUTXOStore) Get(key string) (ActiveUTXO, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if utxo, ok := s.mem[key]; ok {
		return utxo, true
	}
	// Check disk
	var utxo ActiveUTXO
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			valCopy := append([]byte(nil), val...) // ✅ Safe copy
			return json.Unmarshal(valCopy, &utxo)
		})
	})
	if err == nil {
		return utxo, true
	}
	return ActiveUTXO{}, false
}

func (s *ActiveUTXOStore) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.mem[key]; ok {
		delete(s.mem, key)
	} else {
		// Delete from disk
		err := s.db.Update(func(txn *badger.Txn) error {
			return txn.Delete([]byte(key))
		})
		if err != nil && err != badger.ErrKeyNotFound {
			fmt.Fprintf(os.Stderr, "[Delete] Failed to delete key %s from disk: %v\n", key, err)
		}
	}
}

func (s *ActiveUTXOStore) offloadOldest(fraction float64) {
	numToOffload := int(float64(len(s.order)) * fraction)
	if numToOffload == 0 {
		return
	}
	batch := s.order[:numToOffload]
	s.order = s.order[numToOffload:]

	const chunkSize = 10_000 // Number of entries per mini-batch
	for i := 0; i < len(batch); i += chunkSize {
		end := i + chunkSize
		if end > len(batch) {
			end = len(batch)
		}
		miniBatch := batch[i:end]

		err := s.db.Update(func(txn *badger.Txn) error {
			for _, key := range miniBatch {
				if utxo, ok := s.mem[key]; ok {
					data, err := json.Marshal(utxo)
					if err != nil {
						return err
					}
					if err := txn.Set([]byte(key), data); err != nil {
						return err
					}
					delete(s.mem, key)
				}
			}
			return nil
		})

		if err != nil {
			fmt.Println("Error offloading to disk:", err)
			break // Exit on failure
		}

		// Optional: Yield to allow other goroutines to run
		// runtime.Gosched()
		// Or, if you want to give breathing room:
		// time.Sleep(10 * time.Millisecond)

		fmt.Printf("[offload] Wrote %d UTXOs to disk (%d/%d)\n", len(miniBatch), end, len(batch))
	}
}

func (s *ActiveUTXOStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.mem)
}

func (s *ActiveUTXOStore) Close() {
	s.db.Close()
}

