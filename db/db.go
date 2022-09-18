package db

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"
)

const MaxKeySize int = 128             // 128 bytes
const MaxValueSize int = (1 << 16) - 1 // 65kb
const MaxFileSize int = 1 << 22        // 4mb

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

	for _, f := range datafiles {
		var id int64
		fmt.Sscanf(f.Name(), "%d.dt", &id)

		go func(fname string, idd int64) {
		}(f.Name(), id)
	}

	wg.Wait()
	return nil
}

func (db *DB) Close() {
	db.active.Close()

	for _, f := range db.dfiles {
		f.Close()
	}
}
