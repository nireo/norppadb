package http_test

import (
	"fmt"
	"net/http"
	"testing"

	norppahttp "github.com/nireo/norppadb/http"
	"github.com/nireo/norppadb/store"
	"github.com/stretchr/testify/require"
)

type testStore struct {
	leaderAddr string
}

func (m *testStore) Get(key []byte) ([]byte, error) {
	return nil, nil
}

func (m *testStore) Put(key, value []byte) error {
	return nil
}

func (m *testStore) Join(id, addr string) error {
	return nil
}

func (m *testStore) Leave(id string) error {
	return nil
}

func (m *testStore) LeaderAddr() string {
	return m.leaderAddr
}

func (m *testStore) GetServers() ([]*store.Server, error) {
	return nil, nil
}

func Test_newService(t *testing.T) {
	st := &testStore{}

	s := norppahttp.New("127.0.0.1:0", st)
	require.NotNil(t, s)

	require.False(t, s.HTTPS(), "http server should be using http not https")
}

func Test_notAllowed(t *testing.T) {
	st := &testStore{}
	s := norppahttp.New("127.0.0.1:0", st)
	require.NotNil(t, s)

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start service")
	}
	defer s.Close()

	host := fmt.Sprintf("http://%s", s.Addr().String())
	client := &http.Client{}
	resp, err := client.Get(host + "/blahblahblah")
	require.NoError(t, err)
	require.Equal(t, resp.StatusCode, http.StatusNotFound)
	resp, err = client.Post(host+"/xxx", "", nil)

	require.NoError(t, err)
	require.Equal(t, resp.StatusCode, http.StatusNotFound)
}

func Test_EnableHTTPS(t *testing.T) {
	st := &testStore{}
	s := norppahttp.New("127.0.0.1:0", st)
	require.NotNil(t, s)

	err := s.EnableHTTPS("DOESFISJEFOSEF", "does not exist :D", "helloworld")
	require.NoError(t, err)
}
