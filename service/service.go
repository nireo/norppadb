package service

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	"github.com/nireo/norppadb/http"
	"github.com/nireo/norppadb/registry"
	"github.com/nireo/norppadb/store"
	"github.com/soheilhy/cmux"
	"github.com/valyala/fasthttp"
)

type Config struct {
	DataDir   string
	BindAddr  string
	Bootstrap bool
	NodeName  string
	JoinAddrs []string
	Port      int
}

type Service struct {
	Config Config

	mux   cmux.CMux
	store *store.Store
	reg   *registry.Registry

	shutdown     bool
	shutdowns    chan struct{}
	shutdownlock sync.Mutex
}

func New(conf Config) (*Service, error) {
	s := &Service{
		Config: conf,
	}

	setups := []func() error{}

	for _, fn := range setups {
		if err := fn(); err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (s *Service) setupMux() error {
	rpcAddr := fmt.Sprintf(":%d", s.Config.Port)
	l, err := net.Listen("tcp", rpcAddr)
	if err != nil {
		return err
	}
	s.mux = cmux.New(l)
	return nil
}

// setupStore sets up the raft store.
func (s *Service) setupStore() error {
	conf := &store.Config{}
	conf.LocalID = raft.ServerID(s.Config.NodeName)
	conf.Bootstrap = s.Config.Bootstrap

	var err error
	s.store, err = store.New(s.Config.DataDir, conf, false)
	if err != nil {
		return err
	}
	if s.Config.Bootstrap {
		_, err = s.store.WaitForLeader(3 * time.Second)
	}
	return err
}

// setupHTTP sets up a HTTP handler to interact with the store.
func (s *Service) setupHTTP() error {
	httpListener := s.mux.Match(cmux.HTTP2())
	httpServer, err := http.New(s.store)
	if err != nil {
		return err
	}

	go func() {
		go fasthttp.Serve(httpListener, httpServer.Handler)
	}()

	return nil
}
