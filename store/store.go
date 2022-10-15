package store

// store package contains all the logic for distributing the database service
// for example the raft implementation. The HTTP endpoints for raft are in a
// different file.

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/nireo/norppadb/db"
	"google.golang.org/protobuf/proto"
)

const (
	raftTimeout      = 10 * time.Second
	leaderWaitDelay  = 100 * time.Millisecond
	appliedWaitDelay = 100 * time.Millisecond
	maxPool          = 5
)

var (
	ErrTimeoutExpired = errors.New("timeout expired")
	ErrNotLeader      = errors.New("not leader")
	ErrJoinSelf       = errors.New("trying to join self")
	ErrValuesAreNil   = errors.New("values are nil")
)

type Config struct {
	Bootstrap          bool
	SnapshotThreshold  uint64
	StrongConsistency  bool
	BindAddr           string
	CommitTimeout      time.Duration
	LocalID            raft.ServerID
	HeartbeatTimeout   time.Duration
	ElectionTimeout    time.Duration
	LeaderLeaseTimeout time.Duration
}

type RaftStore interface {
	Get(key []byte) (val []byte, err error)
	Put(key []byte, val []byte) error
	Leave(id string) error
	Join(id, addr string) error
	GetServers() ([]*Server, error)
	LeaderAddr() string
}

type Store struct {
	conf    *Config
	raft    *raft.Raft
	db      *db.BadgerBackend
	raftdir string
	datadir string
	logger  *log.Logger
}

type applyRes struct {
	err error
	res []byte
}

// dir is the datadir which contains raft data and database data
func New(dir string, conf *Config, logging bool) (*Store, error) {
	st := &Store{conf: conf}
	if err := st.setupdb(dir); err != nil {
		return nil, err
	}
	st.logger = log.New(os.Stdout, "[store] ", log.LstdFlags)

	raftDir := filepath.Join(dir, "raft")
	if err := os.MkdirAll(raftDir, os.ModePerm); err != nil {
		return nil, err
	}
	st.raftdir = raftDir

	st.logger.Printf("binding raft on addr: %s", conf.BindAddr)

	addr, err := net.ResolveTCPAddr("tcp", conf.BindAddr)
	if err != nil {
		return nil, err
	}
	log.Printf("addr: %v", addr.String())

	transport, err := raft.NewTCPTransport(conf.BindAddr, addr, maxPool, raftTimeout, os.Stderr)
	if err != nil {
		return nil, err
	}

	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(raftDir, "raft.db"))
	if err != nil {
		return nil, err
	}

	snapshotStore, err := raft.NewFileSnapshotStore(raftDir, 1, os.Stderr)
	if err != nil {
		return nil, err
	}

	config := raft.DefaultConfig()
	config.SnapshotThreshold = conf.SnapshotThreshold
	config.LocalID = conf.LocalID

	if conf.HeartbeatTimeout != 0 {
		config.HeartbeatTimeout = conf.HeartbeatTimeout
	}

	if conf.ElectionTimeout != 0 {
		config.ElectionTimeout = conf.ElectionTimeout
	}

	if conf.LeaderLeaseTimeout != 0 {
		config.LeaderLeaseTimeout = conf.LeaderLeaseTimeout
	}

	if conf.CommitTimeout != 0 {
		config.CommitTimeout = conf.CommitTimeout
	}

	if !logging {
		// if logging is true raft defaults it to os.Stderr
		config.LogOutput = io.Discard
	}

	st.raft, err = raft.NewRaft(config, st, stableStore, stableStore, snapshotStore, transport)
	if err != nil {
		return nil, err
	}

	hasState, err := raft.HasExistingState(stableStore, stableStore, snapshotStore)
	if err != nil {
		return nil, err
	}

	if conf.Bootstrap && !hasState {
		conf := raft.Configuration{
			Servers: []raft.Server{{
				ID:      config.LocalID,
				Address: raft.ServerAddress(conf.BindAddr),
			}},
		}
		err = st.raft.BootstrapCluster(conf).Error()
	}
	return st, err
}

func (s *Store) setupdb(dir string) error {
	dbPath := filepath.Join(dir, "db")
	if err := os.MkdirAll(dbPath, os.ModePerm); err != nil {
		return err
	}
	s.datadir = dbPath
	var err error
	s.db, err = db.NewBadgerBackend(dbPath)
	return err
}

func (s *Store) Apply(l *raft.Log) interface{} {
	var ac Action

	if err := proto.Unmarshal(l.Data, &ac); err != nil {
		return err
	}

	switch ac.Type {
	case Action_GET:
		if ac.Key == nil {
			return ErrValuesAreNil
		}
		res, err := s.db.Get(ac.Key)
		return &applyRes{res: res, err: err}
	case Action_NEW:
		if ac.Key == nil || ac.Value == nil {
			return ErrValuesAreNil
		}
		return &applyRes{res: nil, err: s.db.Put(ac.Key, ac.Value)}
	}

	return nil
}

func (s *Store) Put(key, value []byte) error {
	if !s.IsLeader() {
		return ErrNotLeader
	}

	res, err := s.apply(Action_NEW, key, value)
	if err != nil {
		return err
	}

	r := res.(*applyRes)
	return r.err
}

type snapshot struct {
	start        time.Time
	lastSnapshot time.Time
	db           *badger.DB
}

func (f *Store) Snapshot() (raft.FSMSnapshot, error) {
	return &snapshot{
		db:           f.db.DB,
		start:        time.Now(),
		lastSnapshot: f.db.LastSnapshot,
	}, nil
}

func (f *Store) Restore(rc io.ReadCloser) error {
	buf := new(bytes.Buffer)
	gz, err := gzip.NewReader(rc)
	if err != nil {
		return err
	}

	if _, err := io.Copy(buf, gz); err != nil {
		return err
	}

	if err := gz.Close(); err != nil {
		return err
	}

	return f.db.Load(bytes.NewReader(buf.Bytes()))
}

func (f *snapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		var buf1 bytes.Buffer
		_, err := f.db.Backup(&buf1, uint64(f.lastSnapshot.Unix()))
		if err != nil {
			return err
		}

		var buf bytes.Buffer
		gz, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		if err != nil {
			return err
		}

		if _, err := gz.Write(buf1.Bytes()); err != nil {
			return err
		}

		if err := gz.Close(); err != nil {
			return err
		}

		if _, err := sink.Write(buf.Bytes()); err != nil {
			return err
		}

		return sink.Close()
	}()

	if err != nil {
		sink.Cancel()
		return err
	}
	return nil
}

func (s *snapshot) Release() {
}

// LeaderAddr returns the leader node address.
func (s *Store) LeaderAddr() string {
	return string(s.raft.Leader())
}

// WaitForLeader waits until a leader is elected.
func (s *Store) WaitForLeader(timeout time.Duration) (string, error) {
	ticker := time.NewTicker(leaderWaitDelay)
	defer ticker.Stop()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-ticker.C:
			l := s.LeaderAddr()
			if l != "" {
				return l, nil
			}
		case <-timer.C:
			return "", ErrTimeoutExpired
		}
	}
}

// IsLeader returns a value indicating if a given store is the leader.
func (s *Store) IsLeader() bool {
	return s.raft.State() == raft.Leader
}

func (s *Store) Close() error {
	f := s.raft.Shutdown()
	if err := f.Error(); err != nil {
		return err
	}
	return s.db.Close()
}

// Leave removes a node with ID from the raft cluster.
func (s *Store) Leave(id string) error {
	s.logger.Printf("received request to remove node %s", id)
	if !s.IsLeader() {
		return ErrNotLeader
	}

	if err := s.remove(id); err != nil {
		s.logger.Printf("failed to remove node %s: %s", id, err.Error())
		return err
	}

	s.logger.Printf("node %s removed successfully", id)
	return nil
}

func (s *Store) apply(ty Action_ACTION_TYPE, key, value []byte) (any, error) {
	action := &Action{
		Type:  ty,
		Key:   key,
		Value: value,
	}

	b, err := proto.Marshal(action)
	if err != nil {
		return nil, err
	}

	f := s.raft.Apply(b, raftTimeout)
	if f.Error() != nil {
		if f.Error() == raft.ErrNotLeader {
			return nil, ErrNotLeader
		}
		return nil, f.Error()
	}

	r := f.Response()
	if err, ok := r.(error); ok {
		return nil, err
	}
	return r, nil
}

func (s *Store) remove(id string) error {
	if !s.IsLeader() {
		return ErrNotLeader
	}

	f := s.raft.RemoveServer(raft.ServerID(id), 0, 0)
	if f.Error() != nil {
		if f.Error() == raft.ErrNotLeader {
			return ErrNotLeader
		}
		return f.Error()
	}

	return nil
}

// Join handles a given node joining the whole raft cluster. The joining has to be done
// using the leader node.
func (s *Store) Join(id, addr string) error {
	s.logger.Printf("received request from node with ID %s, at %s, to join this node", id, addr)
	// only can join the leader
	if !s.IsLeader() {
		return ErrNotLeader
	}
	serverID := raft.ServerID(id)
	serverAddr := raft.ServerAddress(addr)
	if serverID == s.conf.LocalID {
		return ErrJoinSelf
	}

	f := s.raft.GetConfiguration()
	if err := f.Error(); err != nil {
		s.logger.Printf("failed to get raft configuration: %v", err)
		return err
	}

	for _, srv := range f.Configuration().Servers {
		if srv.ID == serverID || srv.Address == serverAddr {
			if srv.ID == serverID && srv.Address == serverAddr {
				return nil
			}

			if err := s.remove(id); err != nil {
				s.logger.Printf("failed to remove node %s: %v", id, err)
				return err
			}

			s.logger.Printf("removed node %s prior to rejoin with changed ID or addr", id)
		}
	}

	addf := s.raft.AddVoter(serverID, serverAddr, 0, 0)
	if err := addf.Error(); err != nil {
		if err == raft.ErrNotLeader {
			return ErrNotLeader
		}
		return addf.Error()
	}
	s.logger.Printf("node with ID %s, at %s, joined successfully", id, addr)
	return nil
}

// Get finds a given key from the raft cluster. If StrongConsistency is enabled,
// the get request will be redirected to the leader node. Otherwise the value is
// read from the given node. Getting from a non-leader node means that the value
// might be old.
func (s *Store) Get(key []byte) ([]byte, error) {
	// query from the leader
	if s.conf.StrongConsistency {
		if !s.IsLeader() {
			return nil, ErrNotLeader
		}

		res, err := s.apply(Action_GET, key, nil)
		if err != nil {
			return nil, err
		}

		r := res.(*applyRes)
		return r.res, r.err
	}
	return s.db.Get(key)
}

// GetServers returns all of the servers that belong to the raft cluster.
func (s *Store) GetServers() ([]*Server, error) {
	future := s.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return nil, err
	}
	var servers []*Server
	for _, server := range future.Configuration().Servers {
		servers = append(servers, &Server{
			Id:       string(server.ID),
			RpcAddr:  string(server.Address),
			IsLeader: s.raft.Leader() == server.Address,
		})
	}
	return servers, nil
}

// LeaderID returns the node ID of the leader node.
func (s *Store) LeaderID() (string, error) {
	addr := s.LeaderAddr()

	conf := s.raft.GetConfiguration()
	if err := conf.Error(); err != nil {
		return "", err
	}

	for _, srv := range conf.Configuration().Servers {
		if srv.Address == raft.ServerAddress(addr) {
			return string(srv.ID), nil
		}
	}

	return "", nil
}

// CheckRaftConfig checks that a given raft config is valid. This
// is used for recovering a raft cluster.
func CheckRaftConfig(conf raft.Configuration) error {
	ids := make(map[raft.ServerID]bool)
	addrs := make(map[raft.ServerAddress]bool)
	var voters int

	for _, srv := range conf.Servers {
		if srv.ID == "" {
			return fmt.Errorf("empty ID in configuration: %v", conf)
		}

		if srv.Address == "" {
			return fmt.Errorf("empty address in configuration: %v", srv)
		}

		if ids[srv.ID] {
			return fmt.Errorf("found duplicate ID in configuration: %v", srv.ID)
		}

		ids[srv.ID] = true
		if addrs[srv.Address] {
			return fmt.Errorf("found duplicate address in configuration: %v", srv.Address)
		}

		addrs[srv.Address] = true
		if srv.Suffrage == raft.Voter {
			voters++
		}
	}

	if voters == 0 {
		return fmt.Errorf("need at least one voter in config")
	}

	return nil
}

// GetConfig handles the GetConfiguration future and returns the raft config.
func (s *Store) GetConfig() (raft.Configuration, error) {
	conf := s.raft.GetConfiguration()
	if err := conf.Error(); err != nil {
		return raft.Configuration{}, err
	}

	return conf.Configuration(), nil
}
