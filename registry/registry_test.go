package registry_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/serf/serf"
	"github.com/nireo/norppadb/registry"
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

type handler struct {
	joins  chan map[string]string
	leaves chan string
}

func (h *handler) Join(id, addr string) error {
	if h.joins != nil {
		h.joins <- map[string]string{
			"id":   id,
			"addr": addr,
		}
	}
	return nil
}

func (h *handler) Leave(id string) error {
	if h.leaves != nil {
		h.leaves <- id
	}
	return nil
}

func setupMember(t *testing.T, members []*registry.Registry) (
	[]*registry.Registry, *handler,
) {
	id := len(members)
	port, _ := getFreePort()
	addr := fmt.Sprintf("%s:%d", "127.0.0.1", port)
	tags := map[string]string{
		"rpc_addr": addr,
	}

	c := registry.Config{
		NodeName: fmt.Sprintf("%d", id),
		BindAddr: addr,
		Tags:     tags,
	}

	h := &handler{}
	if len(members) == 0 {
		h.joins = make(chan map[string]string, 3)
		h.leaves = make(chan string, 3)
	} else {
		c.StartJoinAddrs = []string{
			members[0].BindAddr,
		}
	}

	r, err := registry.New(h, c)
	require.NoError(t, err)
	members = append(members, r)
	return members, h
}

func TestRegistry(t *testing.T) {
	m, handler := setupMember(t, nil)
	m, _ = setupMember(t, m)
	m, _ = setupMember(t, m)

	require.Eventually(t, func() bool {
		return len(handler.joins) == 2 &&
			len(m[0].Members()) == 3 &&
			len(handler.leaves) == 0
	}, 3*time.Second, 250*time.Millisecond)
	require.NoError(t, m[2].Leave())
	require.Eventually(t, func() bool {
		return len(handler.joins) == 2 &&
			len(m[0].Members()) == 3 &&
			serf.StatusLeft == m[0].Members()[2].Status &&
			len(handler.leaves) == 1
	}, 3*time.Second, 250*time.Millisecond)
	require.Equal(t, fmt.Sprintf("%d", 2), <-handler.leaves)
}
