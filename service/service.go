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
)

type Config struct {
	DataDir   string
	BindAddr  string
	Bootstrap bool
	NodeName  string
	JoinAddrs []string
	Port      int
}

// HTTPAddr takes the BindAddr's host and adds the port where the HTTP
// server is supposed to be hosted in the BindAddr's port's place.
func (c *Config) HTTPAddr() (string, error) {
	host, _, err := net.SplitHostPort(c.BindAddr)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%d", host, c.Port), nil
}

type Service struct {
	Config Config

	mux   cmux.CMux
	store *store.Store
	reg   *registry.Registry
	http  *http.Server

	shutdown     bool
	shutdowns    chan struct{}
	shutdownlock sync.Mutex
}

// New creates a Service instance and runs all of the needed setup functions to
// get the service running.
func New(conf Config) (*Service, error) {
	s := &Service{
		Config: conf,
	}

	setups := []func() error{
		s.setupMux,
		s.setupStore,
		s.setupHTTP,
		s.setupRegistry,
	}

	for _, fn := range setups {
		if err := fn(); err != nil {
			return nil, err
		}
	}
	go s.serve()

	return s, nil
}

// setupMux sets up the connection multiplexer such that Raft and HTTP communications
// happen on the same port.
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
	httpListener := s.mux.Match(cmux.Any())

	addr, err := s.Config.HTTPAddr()
	if err != nil {
		return err
	}

	s.http = http.New(addr, s.store)
	if err := s.http.StartWithListener(httpListener); err != nil {
		return err
	}

	return nil
}

// setupRegistry sets up the cluster registry which handles service discovery.
func (s *Service) setupRegistry() error {
	addr, err := s.Config.HTTPAddr()
	if err != nil {
		return err
	}

	s.reg, err = registry.New(s.store, registry.Config{
		NodeName: s.Config.NodeName,
		BindAddr: s.Config.BindAddr,
		Tags: map[string]string{
			"addr": addr,
		},
		StartJoinAddrs: s.Config.JoinAddrs,
	})

	return err
}

// serve starts the connection multiplexer.
func (s *Service) serve() error {
	if err := s.mux.Serve(); err != nil {
		return err
	}
	return nil
}

// Close shuts down the service and all of its components.
func (s *Service) Close() error {
	s.shutdownlock.Lock()
	defer s.shutdownlock.Unlock()

	if s.shutdown {
		return nil
	}
	s.shutdown = true
	close(s.shutdowns)

	closeFns := []func() error{
		s.reg.Leave,
		func() error {
			s.http.Close()
			return nil
		},
		s.store.Close,
	}

	for _, fn := range closeFns {
		if err := fn(); err != nil {
			return nil
		}
	}

	return nil
}
