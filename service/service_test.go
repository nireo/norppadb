package service_test

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/nireo/norppadb/service"
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

func genNPorts(n int) []int {
	ports := make([]int, n)
	for i := 0; i < n; i++ {
		ports[i], _ = getFreePort()
	}

	return ports
}

func TestService(t *testing.T) {
	var services []*service.Service

	for i := 0; i < 3; i++ {
		ports := genNPorts(2)
		bindaddr := fmt.Sprintf("%s:%d", "127.0.0.1", ports[0])
		rpcPort := ports[1]

		datadir, err := os.MkdirTemp("", "service-test")
		require.NoError(t, err)

		var startJoinAddrs []string
		if i != 0 {
			startJoinAddrs = append(startJoinAddrs, services[0].Config.BindAddr)
		}

		service, err := service.New(service.Config{
			NodeName:  fmt.Sprintf("%d", i),
			Bootstrap: i == 0,
			JoinAddrs: startJoinAddrs,
			BindAddr:  bindaddr,
			DataDir:   datadir,
			Port:      rpcPort,
		})
		require.NoError(t, err)

		services = append(services, service)
	}

	defer func() {
		// cleanup
		for _, s := range services {
			s.Close()
			require.NoError(t, os.RemoveAll(s.Config.DataDir))
		}
	}()

	// give some time for the cluster to setup
	time.Sleep(2 * time.Second)

	// create a key
	leaderAddr, err := services[0].Config.HTTPAddr()
	require.NoError(t, err)

	resp, err := http.Post(
		fmt.Sprintf("http://%s/testkey", leaderAddr),
		"text/plain",
		bytes.NewBuffer([]byte("testval")),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	getresp, err := http.Get(fmt.Sprintf("http://%s/testkey", leaderAddr))
	require.NoError(t, err)
	defer getresp.Body.Close()

	body, err := io.ReadAll(getresp.Body)
	require.NoError(t, err)
	require.Equal(t, []byte("testval"), body)

	time.Sleep(1 * time.Second)

	followerAddr, err := services[1].Config.HTTPAddr()
	require.NoError(t, err)

	getresp2, err := http.Get(fmt.Sprintf("http://%s/testkey", followerAddr))
	require.NoError(t, err)
	defer getresp2.Body.Close()

	body, err = io.ReadAll(getresp2.Body)
	require.NoError(t, err)
	require.Equal(t, []byte("testval"), body)
}
