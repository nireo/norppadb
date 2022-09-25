# norppadb

A fast key-value storage with predictable performance. The internal database design resembels bitcask, which means key meta-data is stored in memory for faster look-ups. The downside of this is that the keys will have to fit into memory.

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
