package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
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
}
