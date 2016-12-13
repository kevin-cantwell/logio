package logio

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/kevin-cantwell/logio/internal/server"
)

var (
	// Debug may be set to true for troubleshooting.
	// The env var LOG_LEVEL=DEBUG is equivalent to setting Debug = true
	Debug  = false
	loglvl = strings.ToUpper(os.Getenv("LOG_LEVEL"))

	// May be nil on init()
	out = func() io.Writer {
		rawurl := os.Getenv("LOGIO_URL")
		if rawurl == "" {
			debug("LOGIO_SERVER unset")
			return nil
		}
		c, err := connectURL(rawurl)
		if err != nil {
			debug(err)
			// We do not return here because the server may become available at some point
		}
		go c.handleWrites()
		// Duplicates all logs to the logio server connection. Subsequent calls to log.SetOutput will break
		// the logio server connection. The file os.Stderr is the default output of the log package.
		log.SetOutput(io.MultiWriter(os.Stderr, c))
		return c
	}()
)

func Output() io.Writer {
	return out
}

func debug(a ...interface{}) {
	if Debug || loglvl == "DEBUG" {
		p := []interface{}{"logio:"}
		p = append(p, a...)
		fmt.Println(p...)
	}
}

type Config struct {
	Username string
	Password string
	Address  string // host:port

	App  string
	Proc string
	Host string
}

type connection struct {
	cfg  Config
	logs chan server.Log

	mu   sync.RWMutex
	conn net.Conn
}

// Write implements the io.Writer interface. Every server.Log sent to Write
// will be enqueued in a buffer for sending to the logio server. If the
// buffer is full, the server.Log will be dropped and an error server.Log will
// be returned
func (c *connection) Write(raw []byte) (int, error) {
	l := server.Log{
		Time: time.Now(),
		Raw:  string(raw),
	}
	select {
	case c.logs <- l:
		return len(raw), nil
	default:
		debug("buffer full")
		return 0, fmt.Errorf("logio buffer full")
	}
}

// Dials the logio server and returns the connection. May be
// used to redial in response to errors.
func (c *connection) dial() (net.Conn, error) {
	conn, err := net.Dial("tcp", c.cfg.Address)
	if err != nil {
		return nil, err
	}
	topic := server.Topic{
		App:  c.cfg.App,
		Proc: c.cfg.Proc,
		Host: c.cfg.Host,
	}
	topicBody, err := json.Marshal(topic)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(topicBody); err != nil {
		return nil, err
	}
	return conn, nil
}

func (c *connection) handleWrites() {
	for l := range c.logs {
		err := c.send(l)
		if err == nil {
			continue
		}
		debug(err)

		// If the connection is bad, then pause for 1s and re-dial
		for {
			time.Sleep(time.Second)
			if conn, err := c.dial(); err != nil {
				debug(err)
				continue
			} else {
				c.mu.Lock()
				c.conn = conn
				c.mu.Unlock()
				break
			}
		}
	}
}

func (c *connection) send(l server.Log) error {
	if c.conn == nil {
		return fmt.Errorf("no connection to logio server")
	}
	formatted, err := json.Marshal(l)
	if err != nil {
		return err
	}
	_, err = c.conn.Write(formatted)
	return err
}

// ConnectURL is the manual alternative to setting the connection string
// via env vars. Generally, only the app paramter is needed. Both proc and host
// sensibly default to the process name and hostname, respectively.
//
// logio://user:pass@c:port?app=required&proc=optional&host=optional
func connectURL(rawurl string) (*connection, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "logio" {
		return nil, fmt.Errorf("invalid logio URL scheme: %s", u.Scheme)
	}

	h, p, err := net.SplitHostPort(u.Host)
	if err != nil {
		return nil, fmt.Errorf("invalid logio URL host: %s", u.Host)
	}

	address := net.JoinHostPort(h, p)

	var username, password string
	if u.User != nil {
		username = u.User.Username()
		p, isSet := u.User.Password()
		if isSet {
			password = p
		}
	}

	var app, proc, host string
	if v, ok := u.Query()["app"]; ok {
		app = v[0]
	} else {
		// TODO: Should app have a default? Maybe just `app`?
	}
	if v, ok := u.Query()["proc"]; ok {
		proc = v[0]
	} else {
		// Default is the process name
		proc = path.Base(os.Args[0])
	}
	if v, ok := u.Query()["host"]; ok {
		host = v[0]
	} else {
		// Default is the hostname
		host, _ = os.Hostname()
	}

	return connect(Config{
		Username: username,
		Password: password,
		Address:  address,
		App:      app,
		Proc:     proc,
		Host:     host,
	})
}

func connect(cfg Config) (*connection, error) {
	c := connection{
		cfg:  cfg,
		logs: make(chan server.Log, 1024), // TODO: Make buffer size configurable?
	}
	conn, err := c.dial()
	if err != nil {
		return &c, err
	}
	c.conn = conn
	return &c, nil
}
