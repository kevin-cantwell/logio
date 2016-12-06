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
	Debug  = false
	Config = func() config {
		procname := path.Base(os.Args[0])
		hostname, _ := os.Hostname()

		return config{
			App:      procname,
			ProcName: "", // Left blank by default
			ProcID:   hostname,
		}
	}()
)

func Configure(app, procname, procid string) {
	Config = config{
		App:      app,
		ProcName: procname,
		ProcID:   procid,
	}
}

type config struct {
	App      string
	ProcName string
	ProcID   string
}

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
		addr:         addr,
		outgoingLogs: make(chan Message, 1000),
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
	outgoingLogs chan Message

	mu   sync.RWMutex
	conn net.Conn
}

// Dials the logio server and sets the connection. May be
// used to redial in response to errors. Safe for concurrent
// use.
func (server *logioConn) dial() error {
	server.mu.Lock()
	defer server.mu.Unlock()

	conn, err := net.Dial("tcp", server.addr)
	if err != nil {
		return err
	}
	header := Header{
		App:      Config.App,
		ProcName: Config.ProcName,
		ProcID:   Config.ProcID,
	}
	headerBody, err := json.Marshal(header)
	if err != nil {
		return err
	}
	if _, err := conn.Write(headerBody); err != nil {
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
	logmsg := Message{
		Time: time.Now(),
		Log:  string(raw),
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

func (server *logioConn) send(outgoing Message) error {
	formatted, err := json.Marshal(outgoing)
	if err != nil {
		debug("logio", err)
	}

	server.mu.RLock()
	_, err = server.conn.Write(formatted)
	server.mu.RUnlock()
	return err
}

type Header struct {
	App      string `json:"app"`
	ProcName string `json:"procname"`
	ProcID   string `json:"procid"`

	Tags map[string]string `json:"tags,omitempty"`
}

type Message struct {
	// Time is the time at which the log was written. Required.
	Time time.Time `json:"time"`
	// The entire log line. Required.
	Log string `json:"log"`
}
