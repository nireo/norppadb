package db_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/nireo/norppadb/db"
)

func createTestDB(t *testing.T) (*db.DB, string) {
	file, err := os.MkdirTemp("", "norppadb")
	if err != nil {
		t.Fatal(err)
	}

	db, err := db.Open(file)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.RemoveAll(file)
		db.Close()
	})

	return db, file
}

func Test_entrySerialization(t *testing.T) {
	key := []byte("hello")
	value := []byte("world")
	e := &db.Entry{
		Timestamp: 123123123,
		KeySize:   uint8(len(key)),
		ValueSize: uint16(len(key)),
		Key:       key,
		Value:     value,
	}

	b := e.Serialize()

	newEntry := db.Deserialize(b)

	if newEntry.KeySize != e.KeySize {
		t.Fatalf("key sizes don't match: %d | %d", newEntry.KeySize, e.KeySize)
	}

	if newEntry.ValueSize != e.ValueSize {
		t.Fatalf("value sizes don't match: %d | %d", newEntry.ValueSize, e.ValueSize)
	}

	if newEntry.Timestamp != e.Timestamp {
		t.Fatalf("timestamps don't match: %d | %d", newEntry.Timestamp, e.Timestamp)
	}
}

func Test_dbStartup(t *testing.T) {
	_, dir := createTestDB(t)

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed reading data directory '%s': %s\n", dir, err)
	}

	if len(files) != 1 {
		t.Fatalf("only 1 file should exist in the data directory, but got: %d\n", len(files))
	}
}
