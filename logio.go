package logio

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

var Debug = false

func debug(a ...interface{}) {
	if Debug {
		fmt.Println(a...)
	}
}

func init() {
	addr := os.Getenv("LOGIO_SERVER")
	if addr == "" {
		// TODO: Log something? Panic?
		fmt.Println("logio:", "LOGIO_SERVER unset")
		return
	}
	server := &logioConn{
		addr:     addr,
		writerCh: make(chan []byte),
	}
	if err := server.dial(); err != nil {
		// TODO: Log the err? Panic?
		fmt.Println("logio:", err)
		return
	}
	go server.handleWrites()

	multi := io.MultiWriter(os.Stderr, server)
	log.SetOutput(multi)
}

type logioConn struct {
	addr     string
	writerCh chan []byte

	conn net.Conn
	mu   sync.RWMutex
}

// Dials the logio server and sets the connection. May be
// used to redial in response to errors. Safe for concurrent
// use.
func (server *logioConn) dial() error {
	server.mu.Lock()
	defer server.mu.Unlock()

	conn, err := net.Dial("udp", server.addr)
	if err != nil {
		return err
	}
	server.conn = conn
	return nil
}

// Write implements the io.Writer interface. Every message sent Write
// will be enqueued in a buffer for sending to the logio server. If the
// buffer is full, the message will be dropped and an error message will
// be returned
func (server *logioConn) Write(p []byte) (int, error) {
	select {
	case server.writerCh <- p:
		return len(p), nil
	default:
		// TODO: Add a hook for dropped messages
		return 0, fmt.Errorf("error: logio buffer too full")
	}
}

func (server *logioConn) handleWrites() {
	for write := range server.writerCh {
		err := server.send(write)
		if err == nil {
			continue
		}
		debug("logio:", err)

		// If the connection is bad, then pause until a connection can be re-established
		for _, ok := err.(*net.OpError); ok; _, ok = err.(*net.OpError) {
			time.Sleep(time.Second)
			if err = server.dial(); err != nil {
				debug("logio:", err)
			}
		}
	}
}

func (server *logioConn) send(write []byte) error {
	server.mu.RLock()
	defer server.mu.RUnlock()
	_, err := server.conn.Write(write)
	return err
}
