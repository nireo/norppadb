package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hashicorp/raft"
	norppahttp "github.com/nireo/norppadb/http"
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
	flag.Parse()
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
		jsonmap := make(map[string]interface{})
		jsonmap["id"] = nodeid
		jsonmap["addr"] = raftbind

		b, err := json.Marshal(jsonmap)
		if err != nil {
			log.Fatal(err)
		}

		req, err := http.NewRequest("POST", join+"/join", bytes.NewBuffer(b))
		if err != nil {
			log.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		if req.Response.StatusCode != http.StatusOK {
			log.Fatalf("failed sending join request, got code: %d", req.Response.StatusCode)
		}

		// send a HTTP-request to raft leader to join the cluster.
		return
	}

	// create http server
	srv := norppahttp.New(serveraddr, st)
	if err := srv.Start(); err != nil {
		log.Fatal("error starting server")
	}

	quitCh := make(chan os.Signal, 1)
	signal.Notify(quitCh, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quitCh
}
