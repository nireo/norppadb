package db

import (
	"os"
	"sync"
	"time"
)

// DB contains the logic for handling the database
type DB struct {
	dir        string
	activeFile *File
	oldFiles   map[int64]*File
	mu         sync.RWMutex
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
		activeFile: activeFile,
		dir:        dataDir,
		oldFiles:   make(map[int64]*File),
	}

	return db, nil
}

func (db *DB) Close() {
	db.activeFile.Close()

	for _, f := range db.oldFiles {
		f.Close()
	}
}
