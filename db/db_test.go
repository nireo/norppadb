package db_test

import (
	"testing"

	"github.com/nireo/norppadb/db"
)

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
