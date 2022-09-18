package db

import (
	"fmt"
	"os"
	"path/filepath"
)

type File struct {
	fp     *os.File
	offset int64
	id     int64
}

func (f *File) Close() error {
	return f.fp.Close()
}

func NewFile(path string, id int64) (*File, error) {
	fname := filepath.Join(path, fmt.Sprintf("%d", id), ".data")
	file, err := os.OpenFile(fname, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(fname)
	if err != nil {
		return nil, err
	}
	return &File{file, stat.Size(), id}, nil
}
