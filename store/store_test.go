package store_test

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/nireo/norppadb/store"
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
}
