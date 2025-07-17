package main

import (
	"encoding/json"
	"fmt"
	"github.com/dgraph-io/badger/v4"
	"log"
	"sync"
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

	s.mem[key] = utxo
	s.order = append(s.order, key)

	if len(s.mem) > s.threshold {
		s.offloadOldest(0.3) // remove 30%
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
			return json.Unmarshal(val, &utxo)
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
	delete(s.mem, key)
	// Note: don't shrink order array for simplicity
}

func (s *ActiveUTXOStore) offloadOldest(fraction float64) {
	numToOffload := int(float64(len(s.order)) * fraction)
	if numToOffload == 0 {
		return
	}
	batch := s.order[:numToOffload]

	err := s.db.Update(func(txn *badger.Txn) error {
		for _, key := range batch {
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
	}

	s.order = s.order[numToOffload:]
}

func (s *ActiveUTXOStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.mem)
}

func (s *ActiveUTXOStore) Close() {
	s.db.Close()
}

