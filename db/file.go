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

// returns the value and possible errors
func (f *File) Read(offset int64) ([]byte, error) {
	// read header fast
	buf := make([]byte, HeaderSize)

	if _, err := f.fp.ReadAt(buf, offset); err != nil {
		return nil, err
	}

	// deserialize header information
	hdr := Deserialize(buf)
	offset += HeaderSize + int64(hdr.KeySize)

	val := make([]byte, hdr.ValueSize)
	if _, err := f.fp.ReadAt(val, offset); err != nil {
		return nil, err
	}

	return val, nil
}

func (f *File) Write(e *Entry) error {
	b := e.Serialize()
	if _, err := f.fp.WriteAt(b, f.offset); err != nil {
		return err
	}
	f.offset += e.Size()
	return nil
}
