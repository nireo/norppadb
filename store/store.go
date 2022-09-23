// store package contains all the logic for distributing the database service
// for example the raft implementation. The HTTP endpoints for raft are in a
// different file.

package store

import (
	"errors"
	"io"
	"time"

	"github.com/hashicorp/raft"
	"github.com/nireo/norppadb/db"
)

const (
	raftTimeout      = 10 * time.Second
	leaderWaitDelay  = 100 * time.Millisecond
	appliedWaitDelay = 100 * time.Millisecond
)

var (
	ErrTimeoutExpired = errors.New("timeout expired")
	ErrNotLeader      = errors.New("not leader")
)

type Store struct {
	raft    *raft.Raft
	running bool
	db      *db.DB // the internal database of a node
	raftdir string
}

type fsm struct {
	db *db.DB
}

type snapshot struct {
	db *db.DB
}

var _ raft.FSM = &fsm{}

func (f *fsm) Apply(l *raft.Log) interface{} {
	return nil
}

func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	return &snapshot{db: f.db}, nil
}

func (f *fsm) Restore(snapshot io.ReadCloser) error {
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
