package db

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"
)

const MaxKeySize int = 128             // 128 bytes
const MaxValueSize int = (1 << 16) - 1 // 65kb
const MaxFileSize int64 = 1 << 21      // 2mb

// DB contains the logic for handling the database
type DB struct {
	dir    string
	active *File
	dfiles map[int64]*File
	idx    *kdir
	mu     sync.RWMutex
}

func Open(dataDir string) (*DB, error) {
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
			return nil, err
		}
	}

	activeFile, err := NewFile(dataDir, time.Now().Unix())
	if err != nil {
		return nil, err
	}

	db := &DB{
		active: activeFile,
		dir:    dataDir,
		dfiles: make(map[int64]*File),
		idx:    newKdir(),
	}

	return db, nil
}

func (db *DB) parsedir() error {
	datafiles, err := ioutil.ReadDir(db.dir)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	errChan := make(chan error)

	for _, f := range datafiles {
		var id int64
		fmt.Sscanf(f.Name(), "%d.dt", &id)
		wg.Add(1)

		go func(fname string, idd int64) {
			defer wg.Done()
		}(f.Name(), id)
	}

	close(errChan)
	wg.Wait()
	if len(errChan) != 0 {
		return err
	}

	return nil
}

func (db *DB) Put(key, value []byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	// TODO: check size that we can change the active datafile
	offset := db.active.offset
	if offset > MaxFileSize {
		// we can use the same pointer so that we don't need to open
		// a new file pointer for no reason
		db.dfiles[db.active.id] = db.active

		var err error
		db.active, err = NewFile(db.dir, time.Now().Unix())
		if err != nil {
			return err
		}
		// continue normal operations
	}

	if err := db.active.Write(&Entry{
		Key:       key,
		Value:     value,
		KeySize:   uint8(len(key)),
		ValueSize: uint16(len(value)),
		Timestamp: uint32(time.Now().Unix()),
	}); err != nil {
		return err
	}
	db.idx.Set(key, db.active.id, offset)
	return nil
}

func (db *DB) Get(key []byte) ([]byte, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	info, err := db.idx.Get(key)
	if err != nil {
		return nil, err
	}
	if info.fileid == db.active.id {
		return db.active.Read(info.offset)
	}
	return db.dfiles[info.fileid].Read(info.offset)
}

func (db *DB) Close() error {
	db.active.Close()

	for _, f := range db.dfiles {
		f.Close()
	}
	return nil
}

func (db *DB) Sync() error {
	if err := db.active.fp.Sync(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	errChan := make(chan error)
	for _, f := range db.dfiles {
		wg.Add(1)
		go func(df *File) {
			defer wg.Done()

			if err := df.fp.Sync(); err != nil {
				errChan <- err
				return
			}
		}(f)
	}
	wg.Wait()
	close(errChan)

	if len(errChan) != 0 {
		return errors.New("errors while syncing datafiles")
	}
	return nil
}
