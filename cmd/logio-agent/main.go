package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/kevin-cantwell/logio/internal/server"
	"github.com/kevin-cantwell/resp"
	"github.com/urfave/cli"
)

var (
	// Debug may be set to true for troubleshooting.
	Debug = (os.Getenv("LOG_LEVEL") == "DEBUG")
)

func main() {
	logger := &server.Logger{
		Logger: log.New(os.Stdout, "", log.LstdFlags),
		ID:     fmt.Sprintf("[logio-agent]"),
	}

	app := cli.NewApp()
	app.Name = "logio-agent"
	app.Usage = "Sends logs to a logio server."
	app.Flags = []cli.Flag{
	// cli.BoolFlag{
	//   Name:  "decode, d",
	//   Usage: "Decodes redis protocol. Default is to encode.",
	// },
	// cli.BoolFlag{
	//   Name:  "raw, r",
	//   Usage: "Decodes redis protocol into a raw format. Default is human-readable.",
	// },
	}
	app.Action = func(ctx *cli.Context) error {
		logioURL := os.Getenv("LOGIO_URL")
		if logioURL == "" {
			return errors.New("LOGIO_URL unset")
		}

		config, err := parseConfigURL(logioURL)
		if err != nil {
			return err
		}

		conn, err := net.Dial("tcp", config.Address)
		if err != nil {
			return err
		}

		logioConn := &LogioConn{
			Reader: resp.NewReader(conn),
			Writer: resp.NewWriter(conn),
			log:    logger,
		}

		if err := logioConn.Auth(config.Username, config.Password); err != nil {
			return errors.New("AUTH error: " + err.Error())
		}

		if err := logioConn.App(config.App, config.Proc, config.Host); err != nil {
			return errors.New("APP error: " + err.Error())
		}

		go func() {
			for {
				data, err := logioConn.ReadData()
				if err != nil {
					logger.ERROR(err.Error())
					time.Sleep(time.Second)
					continue
				}
				switch d := data.(type) {
				case resp.Error:
					logger.ERROR(d.Human())
				default:
					logger.INFO(data.Quote())
				}
			}
		}()

		logs := bufio.NewScanner(io.TeeReader(os.Stdin, os.Stdout))
		for logs.Scan() {
			if err := logioConn.Pub(logs.Text()); err != nil {
				return errors.New("PUB error: " + err.Error())
			}
		}
		if err := logs.Err(); err != nil {
			return err
		}
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		logger.ERROR(err)
		os.Exit(1)
	}
}

type LogioConn struct {
	*resp.Reader
	*resp.Writer
	log *server.Logger
}

func (conn *LogioConn) Auth(username, password string) error {
	err := conn.WriteArray(resp.BulkString("AUTH"), resp.BulkString(username), resp.BulkString(password))
	if err != nil {
		return err
	}
	data, err := conn.ReadData()
	if err != nil {
		return err
	}
	switch d := data.(type) {
	case resp.Error:
		return errors.New(d.Human())
	default:
		if data.Raw() != "OK" {
			return errors.New(data.Raw())
		}
		return nil
	}
}

func (conn *LogioConn) App(app, proc, host string) error {
	err := conn.WriteArray(resp.BulkString("APP"), resp.BulkString(app), resp.BulkString(proc), resp.BulkString(host))
	if err != nil {
		return err
	}
	data, err := conn.ReadData()
	if err != nil {
		return err
	}
	switch d := data.(type) {
	case resp.Error:
		return errors.New(d.Human())
	default:
		if data.Raw() != "OK" {
			return errors.New(data.Raw())
		}
		return nil
	}
}

func (conn *LogioConn) Pub(line string) error {
	err := conn.WriteArray(resp.BulkString("PUB"), resp.Integer(time.Now().UnixNano()), resp.BulkString(line))
	return err
}

// Default values for proc and host are sourced from the name of
// this process (`logio-agent`)
func parseConfigURL(logioURL string) (*Config, error) {
	u, err := url.Parse(logioURL)
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

	procname := path.Base(os.Args[0])
	if procname == "logio-agent" {
		procname = "proc"
	}
	hostname, _ := os.Hostname()
	app, proc, host := "app", procname, hostname
	if v, ok := u.Query()["app"]; ok {
		app = v[0]
	}
	if v, ok := u.Query()["proc"]; ok {
		proc = v[0]
	}
	if v, ok := u.Query()["host"]; ok {
		host = v[0]
	}

	return &Config{
		Username: username,
		Password: password,
		Address:  address,
		App:      app,
		Proc:     proc,
		Host:     host,
	}, nil
}

type Config struct {
	Username string
	Password string
	Address  string // host:port

	App  string
	Proc string
	Host string
}
