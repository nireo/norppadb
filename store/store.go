// store package contains all the logic for distributing the database service
// for example the raft implementation. The HTTP endpoints for raft are in a
// different file.

package store

import (
	"github.com/hashicorp/raft"
	"github.com/nireo/norppadb/db"
)

type Store struct {
	raft    *raft.Raft
	running bool
	db      *db.DB // the internal database of a node
	raftdir string
}
