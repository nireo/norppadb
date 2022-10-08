package db

import (
	"io"
	"time"

	"github.com/dgraph-io/badger/v3"
)

type BadgerBackend struct {
	DB           *badger.DB
	LastSnapshot time.Time
}

func NewBadgerBackend(path string) (*BadgerBackend, error) {
	db, err := badger.Open(badger.DefaultOptions(path))
	if err != nil {
		return nil, err
	}

	return &BadgerBackend{
		DB:           db,
		LastSnapshot: time.Now(),
	}, nil
}

func (b *BadgerBackend) Close() error {
	return b.DB.Close()
}

func (b *BadgerBackend) Put(key, value []byte) error {
	return b.DB.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

func (b *BadgerBackend) Get(key []byte) ([]byte, error) {
	var val []byte
	err := b.DB.View(func(txn *badger.Txn) error {
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
	return b.DB.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

func (b *BadgerBackend) Backup(w io.Writer) error {
	_, err := b.DB.Backup(w, uint64(b.LastSnapshot.Unix()))
	if err != nil {
		return err
	}
	b.LastSnapshot = time.Now()
	return err
}

func (b *BadgerBackend) Load(r io.Reader) error {
	return b.DB.Load(r, 4)
}
