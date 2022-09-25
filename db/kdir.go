package db

import (
	"bytes"
	"encoding/gob"
	"errors"
	"sync"
)

var (
	ErrKeyNotFound = errors.New("key not found")
)

type kdir struct {
	rw sync.RWMutex
	mp map[string]kdirItem
}

func newKdir() *kdir {
	return &kdir{
		mp: make(map[string]kdirItem),
	}
}

type kdirItem struct {
	fileid int64
	offset int64
	timestamp int64
}

func (k *kdir) Set(key []byte, fileid, offset, ts int64) {
	k.rw.Lock()
	defer k.rw.Unlock()

	k.mp[string(key)] = kdirItem{fileid, offset, ts}
}

func (k *kdir) Get(key []byte) (kdirItem, error) {
	k.rw.RLock()
	defer k.rw.RUnlock()

	if v, ok := k.mp[string(key)]; ok {
		return v, nil
	}
	return kdirItem{}, ErrKeyNotFound
}

func (k *kdir) GetKeys() chan string {
	ch := make(chan string)
	go func() {
		for k := range k.mp {
			ch <- k
		}
		close(ch)
	}()
	return ch
}

func (k *kdir) ToBytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(k.mp); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
