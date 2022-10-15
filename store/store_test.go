package store_test

import (
	"bytes"
	"fmt"
	"log"
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

func newStore(t *testing.T, port, id int, bootstrap bool) (*store.Store, error) {
	datadir, err := os.MkdirTemp("", "store-test")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.RemoveAll(datadir)
	})
	config := &store.Config{}
	config.BindAddr = fmt.Sprintf("localhost:%d", port)
	config.LocalID = raft.ServerID(fmt.Sprintf("%d", id))
	config.HeartbeatTimeout = 50 * time.Millisecond
	config.ElectionTimeout = 50 * time.Millisecond
	config.LeaderLeaseTimeout = 50 * time.Millisecond
	config.CommitTimeout = 5 * time.Millisecond
	config.SnapshotThreshold = 10000
	config.Bootstrap = bootstrap

	return store.New(datadir, config)
}

func waitForLeaderID(s *store.Store, timeout time.Duration) (string, error) {
	tck := time.NewTicker(100 * time.Millisecond)
	defer tck.Stop()
	tmr := time.NewTimer(timeout)
	defer tmr.Stop()

	for {
		select {
		case <-tck.C:
			id, err := s.LeaderID()
			if err != nil {
				return "", err
			}
			if id != "" {
				return id, nil
			}
		case <-tmr.C:
			return "", fmt.Errorf("timeout expired")
		}
	}
}

func getNPorts(n int) []int {
	ports := make([]int, n)
	for i := 0; i < n; i++ {
		ports[i], _ = getFreePort()
	}
	return ports
}

func TestMultipleNodes(t *testing.T) {
	var stores []*store.Store
	nodeCount := 3
	ports := getNPorts(nodeCount)

	for i := 0; i < nodeCount; i++ {
		store, err := newStore(t, ports[i], i, i == 0)
		if err != nil {
			t.Fatal(err)
		}

		if i != 0 {
			err = stores[0].Join(fmt.Sprintf("%d", i), fmt.Sprintf("localhost:%d", ports[i]))
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

	log.Println(stores[0].LeaderAddr())
}

func TestSingleNode(t *testing.T) {
	port, err := getFreePort()
	require.NoError(t, err)

	store, err := newStore(t, port, 1, true)
	require.NoError(t, err)

	store.WaitForLeader(3 * time.Second)

	require.True(t, store.IsLeader())

	err = store.Put([]byte("hello"), []byte("world"))
	require.NoError(t, err)

	val, err := store.Get([]byte("hello"))
	require.NoError(t, err)

	require.Equal(t, val, []byte("world"))
}

func TestGetServers(t *testing.T) {
	var stores []*store.Store
	nodeCount := 3
	ports := getNPorts(nodeCount)

	for i := 0; i < nodeCount; i++ {
		store, err := newStore(t, ports[i], i, i == 0)
		if err != nil {
			t.Fatal(err)
		}

		if i != 0 {
			err = stores[0].Join(fmt.Sprintf("%d", i), fmt.Sprintf("localhost:%d", ports[i]))
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

	srvs, err := stores[0].GetServers()
	require.NoError(t, err)

	require.Equal(t, 3, len(srvs))
}

func TestCheckConf(t *testing.T) {
	var stores []*store.Store
	nodeCount := 3
	ports := getNPorts(nodeCount)

	for i := 0; i < nodeCount; i++ {
		store, err := newStore(t, ports[i], i, i == 0)
		if err != nil {
			t.Fatal(err)
		}

		if i != 0 {
			err = stores[0].Join(fmt.Sprintf("%d", i), fmt.Sprintf("localhost:%d", ports[i]))
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

	conf, err := stores[0].GetConfig()
	require.NoError(t, err)

	err = store.CheckRaftConfig(conf)
	require.NoError(t, err)
}
