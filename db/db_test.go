package db_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/nireo/norppadb/db"
	"github.com/nireo/norppadb/messages"
	"github.com/stretchr/testify/require"
)

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

func genRandomPairs(amount, stringSize int) []*messages.KVPair {
	alphabet := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	alen := len(alphabet)
	pairs := make([]*messages.KVPair, 0)
	for i := 0; i < amount; i++ {
		b := make([]byte, stringSize)
		for j := 0; j < stringSize; j++ {
			b[j] = alphabet[rand.Intn(alen)]
		}

		pr := &messages.KVPair{
			Key:   b,
			Value: b,
		}

		pairs = append(pairs, pr)
	}

	return pairs
}

func TestBasicOperations(t *testing.T) {
	pairs := genRandomPairs(32, 10)
	db, _ := createTestBadgerDB(t)

	for _, pr := range pairs {
		err := db.Put(pr.Key, pr.Value)
		require.NoError(t, err)
	}

	for _, pr := range pairs {
		val, err := db.Get(pr.Key)
		require.NoError(t, err)

		require.Equal(t, pr.Value, val)
	}
}

func TestBatchWrite(t *testing.T) {
	pairs := genRandomPairs(16, 10)

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
