package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/raft"
	httpd "github.com/nireo/norppadb/http"
	"github.com/nireo/norppadb/store"
)

type Config struct {
	DataDir    string
	HTTPAddr   string
	AuthFile   string
	X509CACert string
	X509Cert   string
	X509Key    string
	NodeID     string
	RaftAddr   string
	JoinAddr   string
	DiskPath   string
	Boostrap   bool
}

func validateAddr(addr string) error {
	shp, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("HTTP address is not valid: %s", err)
	}

	if ad := net.ParseIP(shp); ad != nil && ad.IsUnspecified() {
		return fmt.Errorf("HTTP address is not routable: (%s)", ad)
	}

	// ok!
	return nil
}

func (c *Config) Validate() error {
	if strings.HasPrefix(strings.ToLower(c.HTTPAddr), "http") {
		return errors.New("HTTP address should not include protocol (http:// or https://)")
	}

	if err := validateAddr(c.HTTPAddr); err != nil {
		return err
	}
	if err := validateAddr(c.RaftAddr); err != nil {
		return err
	}

	if c.JoinAddr != "" {
		addrs := strings.Split(c.JoinAddr, ",")
		for i := range addrs {
			a, err := url.Parse(addrs[i])
			if err != nil {
				return fmt.Errorf("invalid join address '%s': %s", addrs[i], err)
			}

			if a.Host == c.HTTPAddr || addrs[i] == c.HTTPAddr {
				return errors.New("node cannot join itself")
			}
		}
	}

	return nil
}

func (c *Config) HTTPURL() string {
	scheme := "http"
	if c.X509Cert != "" {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s", scheme, c.HTTPAddr)
}

func ParseFlags() (*Config, error) {
	conf := &Config{}

	flag.StringVar(&conf.NodeID, "id", "", "Unique identifier for node.")
	flag.StringVar(&conf.HTTPAddr, "http", "localhost:9000", "HTTP server address to bind to.")
	flag.StringVar(&conf.RaftAddr, "raft", "localhost:50000", "The address to bind raft to.")
	flag.StringVar(&conf.X509CACert, "http-ca-cert", "", "Path to root X.509 certificate")
	flag.StringVar(&conf.X509Cert, "http-cert", "", "Path to X.509 certificate")
	flag.StringVar(&conf.X509Key, "http-key", "", "Path to X.509 key")
	flag.StringVar(&conf.AuthFile, "auth-file", "", "Path to file defining user authentication.")
	flag.StringVar(&conf.JoinAddr, "join", "", "List of addresses to attempt to join.")
	flag.StringVar(&conf.DiskPath, "disk-path", "", "Disk path for the database. If left empty, database will start in memory.")
	flag.BoolVar(&conf.Boostrap, "bootstrap", false, "Bootstrap the cluster.")

	flag.Parse()

	if flag.NArg() < 1 {
		exitWithMessage(1, "data-dir is missing")
	}
	conf.DataDir = flag.Arg(0)

	if flag.NArg() > 1 {
		exitWithMessage(1, "arguments after data-dir not read.")
	}

	if err := conf.Validate(); err != nil {
		return nil, err
	}

	return conf, nil
}

func init() {
	log.SetFlags(log.LstdFlags)
	log.SetOutput(os.Stderr)
	log.SetPrefix("[norppa-server]")
}

func exitWithMessage(code int, errorMsg string) {
	fmt.Fprintf(os.Stderr, "%s\n", errorMsg)
	os.Exit(code)
}

func main() {
	// Parse command line arguments. Easier to take them as command line flags
	// for automation rather than having a separate configuration file.
	conf, err := ParseFlags()
	if err != nil {
		exitWithMessage(1, fmt.Sprintf("failed reading flags or validating config: %s", err))
	}

	// Create raft store.
	storeConfig := &store.Config{}
	storeConfig.BindAddr = conf.RaftAddr
	storeConfig.LocalID = raft.ServerID(conf.NodeID)
	storeConfig.HeartbeatTimeout = 50 * time.Millisecond
	storeConfig.ElectionTimeout = 50 * time.Millisecond
	storeConfig.LeaderLeaseTimeout = 50 * time.Millisecond
	storeConfig.CommitTimeout = 5 * time.Millisecond
	storeConfig.SnapshotThreshold = 10000
	storeConfig.Bootstrap = conf.Boostrap

	st, err := store.New(conf.DataDir, storeConfig, false)
	if err != nil {
		exitWithMessage(1, fmt.Sprintf("failed starting store: %s", err))
	}

	if conf.JoinAddr != "" {
		// Create request body.
		jsonmap := make(map[string]interface{})
		jsonmap["id"] = conf.NodeID
		jsonmap["addr"] = conf.RaftAddr

		b, err := json.Marshal(jsonmap)
		if err != nil {
			exitWithMessage(1, "cannot marshal request body into json")
		}

		// Send join request to the leader such that the
		req, err := http.NewRequest("POST", conf.JoinAddr+"/join", bytes.NewBuffer(b))
		if err != nil {
			exitWithMessage(1, "cannot create HTTP post request.")
		}

		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			exitWithMessage(1, "cannot send join request")
		}

		defer resp.Body.Close()

		// If joining failed end the program since the functionality will differ from
		// what is expected.
		if resp.StatusCode != http.StatusOK {
			exitWithMessage(1, fmt.Sprintf(
				"failed sending join request, got code: %d", req.Response.StatusCode))
		}
	}

	srv := httpd.New(conf.HTTPURL(), st)

	// Enable authentication if conf.AuthFile field has been set. If the file is
	// invalid we should inform the user since that leads to unwanted consequences
	// if the files are wrong.
	if conf.AuthFile != "" {
		if err := srv.EnableAuth(conf.AuthFile); err != nil {
			exitWithMessage(1, fmt.Sprintf("failed reading auth file: %s", err))
		}
	}

	// ignore the error, because this function only populates the key/cert paths
	// in the service struct. it also checks that if the files exist, but that isn't
	// that necessary now. When we start the service, having invalid files will bring
	// up the error.
	srv.EnableHTTPS(conf.X509Cert, conf.X509CACert, conf.X509Key)

	// Start the HTTP service.
	if err := srv.Start(); err != nil {
		log.Fatal("error starting server")
	}

	// Create a syscall listener for qutting the server. Since the service runs in a goroutine. Without this
	// the service would close.
	quitCh := make(chan os.Signal, 1)
	signal.Notify(quitCh, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quitCh
}
