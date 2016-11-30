package logio

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"sync"
	"time"
)

var (
	Debug = false
	Tags  map[string]string
)

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
	hostname, _ := os.Hostname()
	server := &logioConn{
		addr:         addr,
		hostname:     hostname,
		procname:     path.Base(os.Args[0]),
		outgoingLogs: make(chan LogMessage, 1000),
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
	addr         string
	hostname     string
	procname     string
	outgoingLogs chan LogMessage

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
func (server *logioConn) Write(raw []byte) (int, error) {
	logmsg := LogMessage{
		Time:     time.Now(),
		Log:      string(raw),
		Hostname: server.hostname,
		Procname: server.procname,
		Tags:     Tags,
	}
	select {
	case server.outgoingLogs <- logmsg:
		return len(raw), nil
	default:
		// TODO: Add a hook for dropped messages?
		return 0, fmt.Errorf("error: logio buffer too full")
	}
}

func (server *logioConn) handleWrites() {
	for outgoing := range server.outgoingLogs {
		err := server.send(outgoing)
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

func (server *logioConn) send(outgoing LogMessage) error {
	formatted, err := json.Marshal(outgoing)
	if err != nil {
		debug("logio", err)
	}
	server.mu.RLock()
	defer server.mu.RUnlock()
	_, err = server.conn.Write(formatted)
	return err
}

type LogMessage struct {
	// Time is the time at which the log was written. Required.
	Time time.Time `json:"time"`
	// The entire log line. Required.
	Log string `json:"log"`
	// IP is set by the server
	IP string `json:"ip,omitempty"`
	// Hostname from `os.Hostname() (string, error)`
	Hostname string `json:"hostname,omitempty"`
	// Procname is taken as `path.Base(os.Args[0])`
	Procname string `json:"procname,omitempty"`
	// User-supplied key-value tags
	Tags map[string]string `json:"tags,omitempty"`
}
