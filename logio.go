package logio

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
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
		return
	}
	serverConn := &logioConn{
		addr:  addr,
		logch: make(chan []byte),
	}
	go serverConn.sendAll()

	multi := io.MultiWriter(os.Stderr, serverConn)
	log.SetOutput(multi)
}

type logioConn struct {
	addr string
	conn net.Conn
	mu   sync.RWMutex

	logch chan []byte
}

func (reconn *logioConn) Write(logmsg []byte) (int, error) {
	select {
	case reconn.logch <- logmsg:
		return len(logmsg), nil
	default:
		return 0, fmt.Errorf("error: logio buffer too full")
	}
}

func (reconn *logioConn) sendAll() {
	if err := reconn.dial(); err != nil {
		debug("logio:", "error dialing udp conn:", err)
		return
	}
	for logmsg := range reconn.logch {
		if err := reconn.robustlySend(logmsg); err != nil {
			debug("logio:", err)
		}
	}
}

func (reconn *logioConn) robustlySend(logmsg []byte) error {
	_, err := reconn.conn.Write(logmsg)
	switch err.(type) {
	case *net.OpError:
		if err := reconn.dial(); err != nil {
			return err
		}
		return reconn.robustlySend(logmsg)
	}
	return err
}

func (reconn *logioConn) dial() error {
	reconn.mu.Lock()
	defer reconn.mu.Unlock()

	conn, err := net.Dial("udp", reconn.addr)
	if err != nil {
		return err
	}
	reconn.conn = conn
	return nil
}
