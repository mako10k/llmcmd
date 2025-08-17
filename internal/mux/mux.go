package mux

import (
	"bufio"
	"encoding/binary"
	"io"
	"sync"
)

// Conn implements a 4-byte big-endian length-prefixed framed connection.
// Thread-safe writes; single-reader expected.
type Conn struct {
	rwc io.ReadWriteCloser
	r   *bufio.Reader
	w   *bufio.Writer
	muW sync.Mutex
}

func New(rwc io.ReadWriteCloser) *Conn {
	return &Conn{rwc: rwc, r: bufio.NewReader(rwc), w: bufio.NewWriter(rwc)}
}

func (c *Conn) WriteFrame(payload []byte) error {
	c.muW.Lock()
	defer c.muW.Unlock()
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(payload)))
	if _, err := c.w.Write(lenBuf[:]); err != nil { return err }
	if _, err := c.w.Write(payload); err != nil { return err }
	return c.w.Flush()
}

func (c *Conn) ReadFrame() ([]byte, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(c.r, lenBuf[:]); err != nil { return nil, err }
	n := binary.BigEndian.Uint32(lenBuf[:])
	if n == 0 { return []byte{}, nil }
	buf := make([]byte, n)
	if _, err := io.ReadFull(c.r, buf); err != nil { return nil, err }
	return buf, nil
}

func (c *Conn) Close() error { return c.rwc.Close() }
