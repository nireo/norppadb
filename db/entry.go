package db

import (
	"encoding/binary"
)

// TIMESTAMP(uint32) + keysize(uint8|byte) + valuesize(uint16)
const HeaderSize = 4 + 1 + 2

type Entry struct {
	Key       []byte
	Value     []byte
	KeySize   uint8
	ValueSize uint16
	Timestamp uint32
}

func (e *Entry) Size() int64 {
	return int64(e.KeySize) + int64(e.ValueSize) + HeaderSize
}

func (e *Entry) Serialize() []byte {
	kuint16 := uint16(e.KeySize)
	buf := make([]byte, HeaderSize+kuint16+e.ValueSize)
	binary.LittleEndian.PutUint32(buf[0:4], e.Timestamp)
	buf[4] = e.KeySize
	binary.LittleEndian.PutUint16(buf[5:7], e.ValueSize)
	copy(buf[HeaderSize:HeaderSize+kuint16], e.Key)
	copy(buf[HeaderSize+kuint16:], e.Value)

	return buf
}

func Deserialize(b []byte) *Entry {
	e := &Entry{}
	e.Timestamp = binary.LittleEndian.Uint32(b[0:4])
	e.KeySize = b[4]
	e.ValueSize = binary.LittleEndian.Uint16(b[5:7])
	return e
}
