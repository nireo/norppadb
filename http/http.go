package http

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/nireo/norppadb/db"
	"github.com/nireo/norppadb/store"
)

type Server struct {
	store     store.RaftStore
	ln        net.Listener
	addr      string
	closeChan chan struct{}
}

func New(addr string, store store.RaftStore) *Server {
	return &Server{
		store:     store,
		addr:      addr,
		closeChan: make(chan struct{}),
	}
}

func (s *Server) Start() error {
	server := http.Server{
		Handler: s,
	}

	var err error
	s.ln, err = net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	go func() {
		// start listening on thread
		if err := server.Serve(s.ln); err != nil {
			log.Printf("failed to read listener")
		}
	}()
	return nil
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
			fmt.Sprintf("%s%s", leaderAddr, r.URL.Path), http.StatusMovedPermanently)
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
				fmt.Sprintf("%s%s", leaderAddr, r.URL.Path), http.StatusMovedPermanently)
			return
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet &&
		r.Method != http.MethodDelete &&
		r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

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
	}
}
