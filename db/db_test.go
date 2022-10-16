package db_test

import (
	"bytes"
	"errors"
	"log"
	"math/rand"
	"os"
	"sync"
	"testing"

	"github.com/nireo/norppadb/db"
	"github.com/stretchr/testify/require"
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
		db.Close()
		os.RemoveAll(file)
	})

	return db, file
}

func createTestBadgerDB(t *testing.T) (*db.BadgerBackend, string) {
	file, err := os.MkdirTemp("", "norppadb")
	require.NoError(t, err)

	db, err := db.NewBadgerBackend(file)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(file)
	})
	return db, file
}

type testpair struct {
	key   []byte
	value []byte
}

func genRandomPairs(amount, stringSize int) []testpair {
	alphabet := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	alen := len(alphabet)
	pairs := make([]testpair, amount)
	for i := 0; i < amount; i++ {
		b := make([]byte, stringSize)
		for j := 0; j < stringSize; j++ {
			b[j] = alphabet[rand.Intn(alen)]
		}

		pairs[i].key = b
		pairs[i].value = b
	}
	return pairs
}

func genRandomPairs2(amount, stringSize int) []*db.KVPair {
	alphabet := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	alen := len(alphabet)
	pairs := make([]*db.KVPair, 0)
	for i := 0; i < amount; i++ {
		b := make([]byte, stringSize)
		for j := 0; j < stringSize; j++ {
			b[j] = alphabet[rand.Intn(alen)]
		}

		pr := &db.KVPair{
			Key:   b,
			Value: b,
		}

		pairs = append(pairs, pr)
	}

	return pairs
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

	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed reading data directory '%s': %s\n", dir, err)
	}

	if len(files) != 1 {
		t.Fatalf("only 1 file should exist in the data directory, but got: %d\n", len(files))
	}
}

func Test_dbOperations(t *testing.T) {
	db, _ := createTestDB(t)

	key := []byte("hello")
	value := []byte("world")

	if err := db.Put(key, value); err != nil {
		t.Fatalf("error writing key-value pair to database: %s\n", err)
	}

	val, err := db.Get(key)
	if err != nil {
		t.Fatalf("error getting key from database: %s\n", err)
	}

	if !bytes.Equal(val, value) {
		t.Fatalf("values don't match.\n\tgot: %s\n\twant: %s\n", string(val), string(value))
	}
}

func Test_manyDbOperations(t *testing.T) {
	db, _ := createTestDB(t)

	pairs := genRandomPairs(50, 32)
	var wg sync.WaitGroup
	errChan := make(chan error)

	for _, pr := range pairs {
		wg.Add(1)
		go func(p testpair) {
			defer wg.Done()
			if err := db.Put(p.key, p.value); err != nil {
				errChan <- err
				return
			}

			val, err := db.Get(p.key)
			if err != nil {
				errChan <- err
				return
			}

			if !bytes.Equal(val, p.value) {
				errChan <- errors.New("values don't match")
				return
			}
		}(pr)
	}

	wg.Wait()
	close(errChan)
	if len(errChan) != 0 {
		t.Fatalf("errors exist")
	}
}

func Test_readid(t *testing.T) {
	type tc struct {
		fname    string
		expected int64
	}

	tcs := []tc{
		{"123.data", 123},
		{"90805911.data", 90805911},
		{"1231231233.hint", 1231231233},
		{"INVALID_ID", 0},
	}

	for idx, tc := range tcs {
		if db.ReadID(tc.fname) != tc.expected {
			t.Fatalf("test %d failed: got %d", idx, db.ReadID(tc.fname))
		}
	}
}

func TestBatchWrite(t *testing.T) {
	pairs := genRandomPairs2(16, 10)

	// write pairs in patch
	db, _ := createTestBadgerDB(t)

	err := db.BatchWrite(pairs)
	require.NoError(t, err)

	// if we cannot find one value that means the BatchWrite has failed
	for _, pr := range pairs {
		_, err = db.Get(pr.Key)
		require.NoError(t, err)
	}
}
