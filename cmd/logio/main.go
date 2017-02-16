package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"

	"github.com/kevin-cantwell/logio/internal/server"
	"github.com/kevin-cantwell/resp"
	"github.com/urfave/cli"
)

func main() {
	logger := &server.Logger{
		Logger: log.New(os.Stdout, "", log.LstdFlags),
		ID:     fmt.Sprintf("[logio]"),
	}

	app := cli.NewApp()
	app.Name = "logio-agent"
	app.Usage = "Fetches logs from a logio server."
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "app, a",
			Value: "*",
			Usage: "Glob pattern to denote which app logs to fetch",
		},
		cli.StringFlag{
			Name:  "proc, p",
			Value: "*",
			Usage: "Glob pattern to denote which proc logs to fetch",
		},
		cli.StringFlag{
			Name:  "host, H",
			Value: "*",
			Usage: "Glob pattern to denote which host logs to fetch",
		},
	}
	app.Action = func(ctx *cli.Context) error {
		logioURL := os.Getenv("LOGIO_URL")
		if logioURL == "" {
			return errors.New("LOGIO_URL unset")
		}

		config, err := parseConfigURL(ctx, logioURL)
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

		if err := logioConn.Sub(config.App, config.Proc, config.Host); err != nil {
			return errors.New("SUB error: " + err.Error())
		}

		for {
			data, err := logioConn.ReadData()
			if err != nil {
				return err
			}
			switch d := data.(type) {
			case resp.Error:
				return errors.New(d.Human())
			case resp.Array:
				if len(d) != 5 {
					return errors.New("unexpected message length")
				}
				fmt.Printf("%s %s[%s] %s: %s\n", d[0].Raw(), d[1].Raw(), d[2].Raw(), d[3].Raw(), d[4].Raw())
			default:
				logger.INFO(data.Quote())
			}
		}
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

func (conn *LogioConn) Sub(app, proc, host string) error {
	return conn.WriteArray(resp.BulkString("SUB"), resp.BulkString(app), resp.BulkString(proc), resp.BulkString(host))
}

func parseConfigURL(ctx *cli.Context, logioURL string) (*Config, error) {
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

	return &Config{
		Username: username,
		Password: password,
		Address:  address,
		App:      ctx.String("app"),
		Proc:     ctx.String("proc"),
		Host:     ctx.String("host"),
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
