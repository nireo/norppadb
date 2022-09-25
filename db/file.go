package db

import (
	"fmt"
	"os"
	"path/filepath"
	"unicode"
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
	// format the given timestamp id and create the file.
	fname := filepath.Join(path, fmt.Sprintf("%d.data", id))
	file, err := os.OpenFile(fname, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	// read state to get file size
	stat, err := os.Stat(fname)
	if err != nil {
		return nil, err
	}
	return &File{file, stat.Size(), id}, nil
}

// NewReadOnly creates a read-only file pointer to the datafile.
func NewReadOnly(path string) (*File, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}

	// id and offset don't matter since we are writing to the file
	// those fields are only for writing items in the key directory
	// and file.
	return &File{file, 0, 0}, nil
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

// ReadHeader only reads the header of the entry, such that we don't need to
// read the whole value where it isn't necessary.
func (f *File) ReadHeader(offset int64) (*Entry, error) {
	buf := make([]byte, HeaderSize)
	if _, err := f.fp.ReadAt(buf, offset); err != nil {
		return nil, err
	}

	e := Deserialize(buf)
	key := make([]byte, e.KeySize)
	if _, err := f.fp.ReadAt(buf, offset + int64(e.KeySize)); err != nil {
		return nil, err
	}

	e.Key = key
	return e, nil
}

func (f *File) Write(e *Entry) error {
	b := e.Serialize()
	if _, err := f.fp.WriteAt(b, f.offset); err != nil {
		return err
	}
	f.offset += e.Size()
	return nil
}

// reads file id(timestamp) from name e.g: 123123123.data -> 123123123
func ReadID(fname string) int64 {
	var res int64

	for _, c := range fname {
		if !unicode.IsDigit(c) {
			// we encountered the filename
			break
		}
		res *= 10
		res += (int64(c) - '0')
	}

	return res
}
