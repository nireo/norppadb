package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hashicorp/raft"
	"github.com/nireo/norppadb/http"
	"github.com/nireo/norppadb/store"
)

var (
	serveraddr string
	datadir    string
	raftbind   string
	nodeid     string
	join       string
)

func init() {
	flag.StringVar(&serveraddr, "addr", ":9000", "server listening address")
	flag.StringVar(&datadir, "datadir", "./", "directory where database data is stored")
	flag.StringVar(&raftbind, "raftbind", "localhost:51231", "raft transport bind addr")
	flag.StringVar(&nodeid, "id", "", "id of the node")
	flag.StringVar(&join, "join", "", "join some existing cluster (if left empty bootstrap cluster)")
}

func main() {
	// create store
	config := &store.Config{}
	config.Raft.BindAddr = raftbind
	config.Raft.LocalID = raft.ServerID(nodeid)
	config.Raft.HeartbeatTimeout = 50 * time.Millisecond
	config.Raft.ElectionTimeout = 50 * time.Millisecond
	config.Raft.LeaderLeaseTimeout = 50 * time.Millisecond
	config.Raft.CommitTimeout = 5 * time.Millisecond
	config.Raft.SnapshotThreshold = 4096
	if join == "" {
		config.Raft.Bootstrap = true
	}

	st, err := store.New(datadir, config)
	if err != nil {
		log.Fatal(err)
	}

	if join != "" {
		// send a HTTP-request to raft leader to join the cluster.
		return
	}

	// create http server
	srv := http.New(serveraddr, st)
	if err := srv.Start(); err != nil {
		log.Fatal("error starting server")
	}

	quitCh := make(chan os.Signal, 1)
	signal.Notify(quitCh, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quitCh
}
