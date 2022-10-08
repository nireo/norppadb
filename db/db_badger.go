package db

import (
	"github.com/dgraph-io/badger/v3"
)

type BadgerBackend struct {
	db *badger.DB
}

func NewBadgerBackend(path string) (*BadgerBackend, error) {
	db, err := badger.Open(badger.DefaultOptions(path))
	if err != nil {
		return nil, err
	}

	return &BadgerBackend{
		db: db,
	}, nil
}

func (b *BadgerBackend) Close() error {
	return b.db.Close()
}

func (b *BadgerBackend) Put(key, value []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

func (b *BadgerBackend) Get(key []byte) ([]byte, error) {
	var val []byte
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		err = item.Value(func(valp []byte) error {
			val = append([]byte{}, valp...)
			return nil
		})
		return err
	})
	return val, err
}

func (b *BadgerBackend) Delete(key []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}
