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
)

type Config struct {
	Raft struct {
		raft.Config
		Bootstrap         bool
		SnapshotThreshold uint64
	}
}

type RaftStore interface {
	Get(key []byte) (val []byte, err error)
	Put(key []byte, val []byte) error
	Delete(key []byte) error
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

func (f *Store) Apply(l *raft.Log) interface{} {
	return nil
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
