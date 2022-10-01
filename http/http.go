package http

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"

	"github.com/nireo/norppadb/db"
	"github.com/nireo/norppadb/store"
)

type Server struct {
	store     store.RaftStore
	ln        net.Listener
	addr      string
	closeChan chan struct{}
	lgr       *log.Logger
	debug     bool // whether to enable debugging routes such as '/debug/pprof'

	// these are public so we can easily configure them without having more
	// function params
	Certfile   string
	Cacertfile string
	Keyfile    string
}

func New(addr string, store store.RaftStore) *Server {
	return &Server{
		store:     store,
		addr:      addr,
		closeChan: make(chan struct{}),
		lgr:       log.New(os.Stderr, "[http]", log.LstdFlags),
	}
}

// Start starts listening on the wanted address. It also handles creating a
// a TLS config if Certfiles, etc... are provided.
func (s *Server) Start() error {
	server := http.Server{
		Handler: s,
	}

	var (
		ln  net.Listener
		err error
	)

	if s.Cacertfile == "" || s.Keyfile == "" {
		// TLS not enabled, scheme is HTTP
		ln, err = net.Listen("tcp", s.addr)
		if err != nil {
			return err
		}
	} else {
		// TLS enabled, scheme is HTTPS
		conf, err := makeconf(s.Certfile, s.Keyfile, s.Cacertfile)
		if err != nil {
			return err
		}

		ln, err = tls.Listen("tcp", s.addr, conf)
		if err != nil {
			return err
		}

		s.lgr.Printf("started a HTTPS server")
	}

	s.ln = ln
	s.closeChan = make(chan struct{})

	go func() {
		// start listening on thread
		if err := server.Serve(s.ln); err != nil {
			s.lgr.Printf("failed to start listening")
		}
	}()
	s.lgr.Printf("listening on %s", s.addr)

	return nil
}

func makeconf(cert, key, cacert string) (*tls.Config, error) {
	config := &tls.Config{
		NextProtos: []string{"h2", "http/1.1"},
		MinVersion: tls.VersionTLS12,
	}

	config.Certificates = make([]tls.Certificate, 1)
	var err error
	config.Certificates[0], err = tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}

	if cacert != "" {
		ans, err := os.ReadFile(cacert)
		if err != nil {
			return nil, err
		}

		config.RootCAs = x509.NewCertPool()

		if ok := config.RootCAs.AppendCertsFromPEM(ans); !ok {
			return nil, fmt.Errorf("failed to parse root certificate in %q", cacert)
		}
	}

	return config, nil
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	key := []byte(r.URL.Path)
	if len(key) > db.MaxKeySize {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	val, err := s.store.Get(key)
	if err != nil {
		leaderAddr := s.store.LeaderAddr()
		if leaderAddr == "" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		http.Redirect(w, r,
			fmt.Sprintf("http://%s%s", leaderAddr, r.URL.Path), http.StatusMovedPermanently)
		return
	}

	w.Write(val)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) put(w http.ResponseWriter, r *http.Request) {
	// set key, use put method since the key maybe already exists so it's more
	// intuitive.
	// key is the url after address and request body is the value
	key := []byte(r.URL.Path)
	val, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(key) > db.MaxKeySize || len(val) > db.MaxValueSize {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = s.store.Put(key, val)
	if err != nil {
		if err == store.ErrNotLeader {
			leaderAddr := s.store.LeaderAddr()
			if leaderAddr == "" {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			http.Redirect(w, r,
				fmt.Sprintf("http://%s%s", leaderAddr, r.URL.Path), http.StatusMovedPermanently)
			return
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) debugpprof(w http.ResponseWriter, r *http.Request) {
	if !s.debug {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	switch r.URL.Path {
	case "/debug/pprof/cmdline":
		pprof.Cmdline(w, r)
	case "/debug/pprof/profile":
		pprof.Profile(w, r)
	case "/debug/pprof/symbol":
		pprof.Symbol(w, r)
	default:
		pprof.Index(w, r)
	}
}

func (s *Server) join(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	m := make(map[string]interface{})
	if err := json.Unmarshal(b, &m); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id, ok := m["id"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	addr, ok := m["addr"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s.lgr.Printf("received request from node with ID %s, at %s, to join this node", id, addr)

	if err := s.store.Join(id.(string), addr.(string)); err != nil {
		if err == store.ErrNotLeader {
			leaderAddr := s.store.LeaderAddr()
			if leaderAddr == "" {
				http.Error(w, "leader not found", http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r,
				fmt.Sprintf("http://%s%s", leaderAddr, r.URL.Path), http.StatusMovedPermanently)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) leave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	m := make(map[string]interface{})
	if err := json.Unmarshal(b, &m); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id, ok := m["id"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := s.store.Leave(id.(string)); err != nil {
		if err == store.ErrNotLeader {
			leaderAddr := s.store.LeaderAddr()
			if leaderAddr == "" {
				http.Error(w, "leader not found", http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r,
				fmt.Sprintf("http://%s%s", leaderAddr, r.URL.Path), http.StatusMovedPermanently)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) Close() {
	close(s.closeChan)
	s.ln.Close()
}

func (s *Server) HTTPS() bool {
	return s.Cacertfile != "" && s.Keyfile != ""
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/join") {
		s.join(w, r)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/leave") {
		s.leave(w, r)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/debug/pprof") {
		s.debugpprof(w, r)
		return
	}

	// handle general cases
	switch r.Method {
	case http.MethodGet:
		s.get(w, r)
		return
	case http.MethodPut:
		s.put(w, r)
		return
	case http.MethodDelete:
		// delete key
		w.WriteHeader(http.StatusNoContent)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
