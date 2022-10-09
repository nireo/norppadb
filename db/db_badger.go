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

type KVPair struct {
	Key   []byte
	Value []byte
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

func (b *BadgerBackend) BatchWrite(pairs []*KVPair) error {
	batch := b.DB.NewWriteBatch()
	defer batch.Cancel()

	for i := range pairs {
		entry := badger.NewEntry(pairs[i].Key, pairs[i].Value)
		if err := batch.SetEntry(entry); err != nil {
			return err
		}
	}

	return batch.Flush()
}

func (b *BadgerBackend) BatchGet(keys [][]byte) ([]*KVPair, error) {
	res := make([]*KVPair, 0)

	for i := range keys {
		val, err := b.Get(keys[i])
		if err != nil || val == nil {
			continue
		}

		res = append(res, &KVPair{
			Key:   keys[i],
			Value: val,
		})
	}
	return res, nil
}

func (b *BadgerBackend) PrefixScan(prefix []byte) ([]*KVPair, error) {
	res := make([]*KVPair, 0)

	b.DB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte("1234")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			v, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			res = append(res, &KVPair{
				Key:   item.KeyCopy(nil),
				Value: v,
			})
		}
		return nil
	})

	return res, nil
}

func (b *BadgerBackend) GarbageCollector() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
	again:
		if err := b.DB.RunValueLogGC(0.7); err == nil {
			goto again
		}
	}
}
