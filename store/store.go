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
	"github.com/nireo/norppadb/messages"
	"google.golang.org/protobuf/proto"
)

const (
	raftTimeout      = 10 * time.Second
	leaderWaitDelay  = 100 * time.Millisecond
	appliedWaitDelay = 100 * time.Millisecond
	maxPool          = 5
	recoveryFile     = "recovery.json"
)

var (
	// ErrTimeoutExpired is for when waiting for a leader node to be elected
	// we run out of the given timeout.
	ErrTimeoutExpired = errors.New("timeout expired")

	// ErrNotLeader is for when a voter/non-voter node tries to execute an action
	// that is leader-only.
	ErrNotLeader = errors.New("not leader")

	// ErrJoinSelf is for  when a node tries to join itself.
	ErrJoinSelf = errors.New("trying to join self")

	// ErrValuesAreNil is for when either a key or value that are given to
	// raft.Apply are nil, even though they should contain []byte data.
	ErrValuesAreNil = errors.New("values are nil")
)

// Config contains the user-configurable values for the store.
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

// RaftStore shows what methods a given Raft cluster should implement.
type RaftStore interface {
	Get(key []byte) (val []byte, err error)
	Put(key []byte, val []byte) error
	Leave(id string) error
	Join(id, addr string) error
	GetServers() ([]*messages.Server, error)
	LeaderAddr() string
}

type Store struct {
	conf         *Config
	raft         *raft.Raft
	db           *db.BadgerBackend
	raftdir      string
	datadir      string
	logger       *log.Logger
	recoveryPath string
}

// applyRes represents a the data returned by raft.Apply()
type applyRes struct {
	err error
	res []byte
}

// New creates a new instance of an store. It sets up the data directory, raft directory
// raft databases, internal database and raft connections. It also checks for possible
// recovery files to recover a failed cluster.
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
	st.recoveryPath = filepath.Join(dir, recoveryFile)

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

	if fileExists(st.recoveryPath) {
		st.logger.Printf("starting to recover from file: %s", st.recoveryPath)

		// handel recovery
		c, err := raft.ReadConfigJSON(st.recoveryPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read recovery files: %s", err.Error())
		}

		if err = RecoverNode(st.datadir, stableStore, stableStore, snapshotStore, transport, c); err != nil {
			return nil, fmt.Errorf("failed to recover node: %s", err.Error())
		}
		st.logger.Printf("node recovered successfully")
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

// setupdb sets up the database on disk.
func (s *Store) setupdb(dir string) error {
	dbPath := filepath.Join(dir, "db")
	if err := os.MkdirAll(dbPath, os.ModePerm); err != nil {
		return err
	}
	s.datadir = dbPath
	var err error
	s.db, err = db.NewBadgerBackend(dbPath)
	if err != nil {
		return err
	}
	go s.db.GarbageCollector()
	return nil
}

// Apply a given raft command. All of the work is done by the applyHelper.
func (s *Store) Apply(l *raft.Log) interface{} {
	return applyHelper(l.Data, &s.db)
}

// applyHelper is different from 'apply' since this is used by the leader to actually
// execute the action given to raft.Apply. 'apply' only sends action information to
// raft.Apply.
func applyHelper(data []byte, dbb **db.BadgerBackend) interface{} {
	db := *dbb
	var ac messages.Action

	if err := proto.Unmarshal(data, &ac); err != nil {
		return err
	}

	switch ac.Type {
	case messages.Action_GET:
		if ac.Key == nil {
			return ErrValuesAreNil
		}
		res, err := db.Get(ac.Key)
		return &applyRes{res: res, err: err}
	case messages.Action_NEW:
		if ac.Key == nil || ac.Value == nil {
			return ErrValuesAreNil
		}
		return &applyRes{res: nil, err: db.Put(ac.Key, ac.Value)}
	case messages.Action_DELETE:
		if ac.Key == nil {
			return ErrValuesAreNil
		}

		return &applyRes{res: nil, err: db.Delete(ac.Key)}
	}

	return nil
}

// Put writes a key-value pair into the cluster. This is a leader-only operation.
func (s *Store) Put(key, value []byte) error {
	if !s.IsLeader() {
		return ErrNotLeader
	}

	res, err := s.apply(messages.Action_NEW, key, value)
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

// Snapshot creates a snapshot of the store.
func (f *Store) Snapshot() (raft.FSMSnapshot, error) {
	return &snapshot{
		db:           f.db.DB,
		start:        time.Now(),
		lastSnapshot: f.db.LastSnapshot,
	}, nil
}

// dbFromSnapshot converts gzip compressed data from a given io.ReadCloser into
// database entries and constructs and database state from that.
func dbFromSnapshot(rc io.ReadCloser) ([]byte, error) {
	buf := new(bytes.Buffer)
	gz, err := gzip.NewReader(rc)
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(buf, gz); err != nil {
		return nil, err
	}

	if err := gz.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Restore takes in the data created by snapshot.Persist() and creates the internal
// database based on that data.
func (f *Store) Restore(rc io.ReadCloser) error {
	b, err := dbFromSnapshot(rc)
	if err != nil {
		return err
	}
	return f.db.Load(bytes.NewReader(b))
}

// Persist writes the state of the database into a raft.SnapshotSink. It also compresses
// the data, since it's more efficient to compress it here and then send it, compared to
// sending all of the bytes at once. It also writes the entries into the database, that
// have been written after the lastSnapshot timestamp.
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

// Release releases snapshot.
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

// Close shuts the Raft cluster and the internal database connection.
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

// apply is a helper to apply a given action in the Raft cluster. It handles errors and
// marshaling action data. It also calls the raft.Apply() function to apply action across
// the whole cluster.
func (s *Store) apply(ty messages.Action_ACTION_TYPE, key, value []byte) (any, error) {
	action := &messages.Action{
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

// remove is a helpher for removing servers from the Raft cluster. Removing nodes from a
// cluster is an leader-only action.
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
// might be old or non-existant.
func (s *Store) Get(key []byte) ([]byte, error) {
	// query from the leader
	if s.conf.StrongConsistency {
		if !s.IsLeader() {
			return nil, ErrNotLeader
		}

		res, err := s.apply(messages.Action_GET, key, nil)
		if err != nil {
			return nil, err
		}

		r := res.(*applyRes)
		return r.res, r.err
	}
	return s.db.Get(key)
}

// Delete deletes the key-value pair with the given key. It is an leader-only
// operation.
func (s *Store) Delete(key []byte) error {
	if !s.IsLeader() {
		return ErrNotLeader
	}

	res, err := s.apply(messages.Action_DELETE, key, nil)
	if err != nil {
		return err
	}

	r := res.(*applyRes)
	return r.err
}

// GetServers returns all of the servers that belong to the raft cluster.
func (s *Store) GetServers() ([]*messages.Server, error) {
	future := s.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return nil, err
	}
	var servers []*messages.Server
	for _, server := range future.Configuration().Servers {
		servers = append(servers, &messages.Server{
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

// fileExists checks if a file a 'path' exists.
func fileExists(path string) bool {
	if _, err := os.Lstat(path); err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

// RecoverNode tries to recover a Raft cluster from a given recovery configuration
// file.
func RecoverNode(dir string, logs raft.LogStore, stable raft.StableStore,
	snaps raft.SnapshotStore, tn raft.Transport, conf raft.Configuration,
) error {
	// ensure that the configuration makes sense
	if err := CheckRaftConfig(conf); err != nil {
		return err
	}

	var (
		snapshotIdx  uint64
		snapshotTerm uint64
	)

	snapshots, err := snaps.List()
	if err != nil {
		return fmt.Errorf("failed to list snapshots: %v", err)
	}

	// find the most recent database bytes.
	var dbBytes []byte
	for _, snapshot := range snapshots {
		// we continue in the hope of finding at least one snapshot
		// and thus shouldn't stop when having errors opening one snapshot

		var src io.ReadCloser
		_, src, err = snaps.Open(snapshot.ID)
		if err != nil {
			continue
		}

		dbBytes, err = dbFromSnapshot(src)
		src.Close()
		if err != nil {
			continue
		}

		snapshotIdx = snapshot.Index
		snapshotTerm = snapshot.Term
		break
	}

	if len(snapshots) > 0 && (snapshotIdx == 0 || snapshotTerm == 0) {
		return fmt.Errorf("failed to restore any of the available snapshots")
	}

	var d *db.BadgerBackend
	if len(dbBytes) == 0 {
		d, err = db.NewBadgerBackendMemory()
	} else {
		d, err = db.NewBadgerBackendMemory()
		if err != nil {
			return err
		}

		if err := d.Load(bytes.NewReader(dbBytes)); err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	defer d.Close()

	lastIdx := snapshotIdx
	lastTerm := snapshotTerm

	lastLogIdx, err := logs.LastIndex()
	if err != nil {
		return fmt.Errorf("failed to find last log: %v", err)
	}

	for idx := snapshotIdx + 1; idx <= lastLogIdx; idx++ {
		var entry raft.Log
		if err := logs.GetLog(idx, &entry); err != nil {
			return fmt.Errorf("failed to get log at idx %d: %v", idx, err)
		}

		if entry.Type == raft.LogCommand {
			applyHelper(entry.Data, &d)
		}

		lastIdx = entry.Index
		lastTerm = entry.Term
	}

	snp := &snapshot{
		db:           d.DB,
		start:        time.Now(),
		lastSnapshot: d.LastSnapshot,
	}

	sink, err := snaps.Create(1, lastIdx, lastTerm, conf, 1, tn)
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %v", err)
	}

	if err = snp.Persist(sink); err != nil {
		return fmt.Errorf("failed to persist snapshot: %v", err)
	}

	if err = sink.Close(); err != nil {
		return fmt.Errorf("failed to finalize snapshot: %v", err)
	}

	firstLogIndex, err := logs.FirstIndex()
	if err != nil {
		return fmt.Errorf("failed to get first log index: %v", err)
	}

	if err := logs.DeleteRange(firstLogIndex, lastLogIdx); err != nil {
		return fmt.Errorf("log compaction failed: %v", err)
	}

	return nil
}
