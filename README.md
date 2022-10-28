# norppadb

[![Go Report Card](https://goreportcard.com/badge/github.com/nireo/norppadb)](https://goreportcard.com/report/github.com/nireo/norppadb)

A robust distributed database with a simple codebase, which only does a few things and does them well.

Distribution is handled by the Raft consensus algorithm and a simple HTTP server is provided to interact with the Raft store.

## Usage

```shell
# startup leader
./norppadb --listen="localhost:9000" --datadir="./data1" --raftbind="localhost:51231" --id="id1"

# connect followers
./norppadb --listen="localhost:9001" --datadir="./data2" --raftbind="localhost:51232" --id="id2" --join="localhost:9000"
./norppadb --listen="localhost:9002" --datadir="./data3" --raftbind="localhost:51233" --id="id3" --join="localhost:9000"

# queries
curl -X POST -d world http://localhost:9000/hello
curl http://localhost:9000/hello
```


## Authentication & Security

NorppaDB supports a simple authentication store which holds permissions that every user has. The permissions are checked using [HTTP Basic scheme](https://datatracker.ietf.org/doc/html/rfc7617). NorppaDB also supports starting up a HTTPS server.


## Server

Using the server binary makes setting up a NorppaDB node a lot easier. Nodes are configured using command line flags. As seen in the "Usage" example not many options are needed to get things up and running. There still are some other options that add some extra functionality and security.

```
Usage of ./server
  -auth-file string
        Path to file defining user authentication.
  -bootstrap
        Bootstrap the cluster.
  -disk-path string
        Disk path for the database. If left empty, database will start in memory.
  -http string
        HTTP server address to bind to. (default "localhost:9000")
  -http-ca-cert string
        Path to root X.509 certificate
  -http-cert string
        Path to X.509 certificate
  -http-key string
        Path to X.509 key
  -id string
        Unique identifier for node.
  -join string
        List of addresses to attempt to join.
  -raft string
        The address to bind raft to. (default "localhost:50000")
```
