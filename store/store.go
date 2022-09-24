// store package contains all the logic for distributing the database service
// for example the raft implementation. The HTTP endpoints for raft are in a
// different file.

package store

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

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
	Raft struct {
		raft.Config
		Bootstrap         bool
		SnapshotThreshold uint64
		StrongConsistency bool
	}
}

type RaftStore interface {
	Get(key []byte) (val []byte, err error)
	Put(key []byte, val []byte) error
	Delete(key []byte) error
	Leave(id string) error
	Join(id, addr string) error
	GetServers() ([]*Server, error)
}

type Store struct {
	conf     *Config
	raft     *raft.Raft
	db       *db.DB // the internal database of a node
	raftdir  string
	datadir  string
	bindaddr string
	nt       *raft.NetworkTransport
}

type snapshot struct {
	db *db.DB
}

type applyRes struct {
	err error
	res []byte
}

// dir is the datadir which contains raft data and database data
func New(dir, bind string) (*Store, error) {
	st := &Store{bindaddr: bind}
	if err := st.setupdb(dir); err != nil {
		return nil, err
	}

	if err := st.setupraft(dir); err != nil {
		return nil, err
	}

	return st, nil
}

func (s *Store) setupdb(dir string) error {
	dbPath := filepath.Join(dir, "db")
	if err := os.MkdirAll(dbPath, os.ModePerm); err != nil {
		return err
	}
	s.datadir = dbPath
	var err error
	s.db, err = db.Open(dbPath)
	return err
}

func (s *Store) setupraft(dir string) error {
	raftDir := filepath.Join(dir, "raft")
	if err := os.MkdirAll(raftDir, os.ModePerm); err != nil {
		return err
	}
	s.raftdir = raftDir

	addr, err := net.ResolveTCPAddr("tcp", s.bindaddr)
	if err != nil {
		return err
	}

	transport, err := raft.NewTCPTransport(s.bindaddr, addr, maxPool, raftTimeout, os.Stderr)
	if err != nil {
		return err
	}

	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(raftDir, "raft.db"))
	if err != nil {
		return err
	}

	snapshotStore, err := raft.NewFileSnapshotStore(raftDir, 1, os.Stderr)
	if err != nil {
		return err
	}

	config := raft.DefaultConfig()
	config.SnapshotThreshold = s.conf.Raft.SnapshotThreshold
	config.LocalID = s.conf.Raft.LocalID

	if s.conf.Raft.HeartbeatTimeout != 0 {
		config.HeartbeatTimeout = s.conf.Raft.HeartbeatTimeout
	}

	if s.conf.Raft.ElectionTimeout != 0 {
		config.ElectionTimeout = s.conf.Raft.ElectionTimeout
	}

	if s.conf.Raft.LeaderLeaseTimeout != 0 {
		config.LeaderLeaseTimeout = s.conf.Raft.LeaderLeaseTimeout
	}

	if s.conf.Raft.CommitTimeout != 0 {
		config.CommitTimeout = s.conf.Raft.CommitTimeout
	}

	s.raft, err = raft.NewRaft(config, s, stableStore, stableStore, snapshotStore, transport)
	if err != nil {
		return err
	}

	hasState, err := raft.HasExistingState(stableStore, stableStore, snapshotStore)
	if err != nil {
		return err
	}

	if s.conf.Raft.Bootstrap && !hasState {
		conf := raft.Configuration{
			Servers: []raft.Server{{
				ID:      config.LocalID,
				Address: s.nt.LocalAddr(),
			}},
		}
		err = s.raft.BootstrapCluster(conf).Error()
	}
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

func (f *Store) Snapshot() (raft.FSMSnapshot, error) {
	return &snapshot{db: f.db}, nil
}

func (f *Store) Restore(snapshot io.ReadCloser) error {
	return nil
}

func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	return sink.Close()
}

func (s *snapshot) Release() {
}

func (s *Store) LeaderAddr() string {
	return string(s.raft.Leader())
}

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

func (s *Store) Leave(id string) error {
	if !s.IsLeader() {
		return ErrNotLeader
	}
	f := s.raft.RemoveServer(raft.ServerID(id), 0, 0)
	if f.Error() != nil {
		if f.Error() == raft.ErrNotLeader {
			return ErrNotLeader
		}
	}
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

func (s *Store) Join(id, addr string) error {
	if !s.IsLeader() {
		return ErrNotLeader
	}
	serverID := raft.ServerID(id)
	serverAddr := raft.ServerAddress(addr)
	if serverID == s.conf.Raft.LocalID {
		return ErrJoinSelf
	}
	f := s.raft.GetConfiguration()
	if err := f.Error(); err != nil {
		return err
	}
	for _, srv := range f.Configuration().Servers {
		if srv.ID == serverID || srv.Address == serverAddr {
			if srv.ID == serverID && srv.Address == serverAddr {
				return nil
			}
			removeFuture := s.raft.RemoveServer(serverID, 0, 0)
			if err := removeFuture.Error(); err != nil {
				return err
			}
		}
	}
	addf := s.raft.AddVoter(serverID, serverAddr, 0, 0)
	if err := addf.Error(); err != nil {
		if err == raft.ErrNotLeader {
			return ErrNotLeader
		}

		return err
	}
	return nil
}

func (s *Store) Get(key []byte) ([]byte, error) {
	// query from the leader
	if s.conf.Raft.StrongConsistency {
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

func (s *Store) Set(key, val []byte) error {
	if !s.IsLeader() {
		return ErrNotLeader
	}
	return nil
}

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
