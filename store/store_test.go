package store_test

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/nireo/norppadb/store"
	"github.com/stretchr/testify/require"
)

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func TestMultipleNodes(t *testing.T) {
	var stores []*store.Store
	nodeCount := 3
	ports := make([]int, nodeCount)

	for i := 0; i < nodeCount; i++ {
		ports[i], _ = getFreePort()
	}

	for i := 0; i < nodeCount; i++ {
		datadir, err := os.MkdirTemp("", "store-test")
		if err != nil {
			t.Fatal(err)
		}
		defer func(dir string) {
			os.RemoveAll(dir)
		}(datadir)
		config := &store.Config{}
		config.Raft.BindAddr = fmt.Sprintf("localhost:%d", ports[i])
		config.Raft.LocalID = raft.ServerID(fmt.Sprintf("%d", i))
		config.Raft.HeartbeatTimeout = 50 * time.Millisecond
		config.Raft.ElectionTimeout = 50 * time.Millisecond
		config.Raft.LeaderLeaseTimeout = 50 * time.Millisecond
		config.Raft.CommitTimeout = 5 * time.Millisecond
		config.Raft.SnapshotThreshold = 4096

		if i == 0 {
			config.Raft.Bootstrap = true
		}

		store, err := store.New(datadir, config)
		if err != nil {
			t.Fatal(err)
		}

		if i != 0 {
			err = stores[0].Join(fmt.Sprintf("%d", i), config.Raft.BindAddr)
			if err != nil {
				t.Fatal(err)
			}
		} else {
			_, err = store.WaitForLeader(3 * time.Second)
			if err != nil {
				t.Fatal(err)
			}
		}
		stores = append(stores, store)
	}

	type pr struct {
		Key   []byte
		Value []byte
	}

	pairs := []pr{
		{Key: []byte("hello"), Value: []byte("world")},
		{Key: []byte("world"), Value: []byte("hello")},
	}

	for _, p := range pairs {
		if err := stores[0].Put(p.Key, p.Value); err != nil {
			t.Fatal(err)
		}

		require.Eventually(t, func() bool {
			for j := 0; j < nodeCount; j++ {
				val, err := stores[j].Get(p.Key)
				if err != nil {
					return false
				}

				if !bytes.Equal(val, p.Value) {
					return false
				}
			}
			return true
		}, 500*time.Millisecond, 50*time.Millisecond)
	}

	err := stores[0].Leave("1")
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	err = stores[0].Put([]byte("hellohello"), []byte("worldworld"))
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	val, err := stores[1].Get([]byte("hellohello"))
	require.Error(t, err)
	require.Nil(t, val)

	val, err = stores[2].Get([]byte("hellohello"))
	require.NoError(t, err)
	require.Equal(t, val, []byte("worldworld"))
}
