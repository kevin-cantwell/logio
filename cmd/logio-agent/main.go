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
	"os/exec"
	"path"
	"time"

	"github.com/kevin-cantwell/logio/internal"
	"github.com/kevin-cantwell/resp"
	"github.com/urfave/cli"
)

var (
	// Debug may be set to true for troubleshooting.
	Debug = (os.Getenv("LOG_LEVEL") == "DEBUG")
)

func main() {
	logger := &internal.Logger{
		Logger: log.New(os.Stdout, "", log.LstdFlags),
		ID:     fmt.Sprintf("[logio-agent]"),
	}

	hostname, _ := os.Hostname()

	app := cli.NewApp()
	app.Name = "logio-agent"
	app.Usage = "Sends logs to a logio server."
	app.UsageText = "The agent may be sourced by an input file, command, or stdin:\n" +
		"\t\t1) logio-agent [opts] -f <logfile>\n" +
		"\t\t2) logio-agent [opts] -c \"<command>\"\n" +
		"\t\t3) logio-agent [opts] < out.log"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "app, a",
			Usage: "The app name that is generating logs. Required.",
		},
		cli.StringFlag{
			Name:  "proc, p",
			Usage: "The process name that is generating logs. Required.",
		},
		cli.StringFlag{
			Name:  "host, o",
			Value: hostname,
			Usage: "The hostname that is generating logs.",
		},
		cli.StringFlag{
			Name:  "username, U",
			Usage: "The Logio username.",
		},
		cli.StringFlag{
			Name:  "password, W",
			Usage: "The Logio password.",
		},
		cli.StringFlag{
			Name:  "server, S",
			Value: "localhost",
			Usage: "The Logio server address.",
		},
		cli.StringFlag{
			Name:  "port, P",
			Value: "7701",
			Usage: "The Logio server port.",
		},
		cli.StringFlag{
			Name:  "file, f",
			Usage: "The log file to read as source input.",
		},
		cli.StringFlag{
			Name:  "command, c",
			Usage: "The command to execute sourcing stdin and stderr as input.",
		},
	}
	app.Action = func(ctx *cli.Context) error {
		// Configure the agent
		var config *Config
		if logioURL := os.Getenv("LOGIO_URL"); logioURL == "" {
			config = &Config{
				Username: ctx.String("username"),
				Password: ctx.String("password"),
				Address:  ctx.String("server") + ":" + ctx.String("port"),
				App:      ctx.String("app"),
				Proc:     ctx.String("proc"),
				Host:     ctx.String("host"),
			}
			if config.App == "" {
				return errors.New("Must specify 'app' option")
			}
			if config.Proc == "" {
				return errors.New("Must specify 'proc' option")
			}
		} else {
			var err error
			config, err = parseConfigURL(logioURL)
			if err != nil {
				return err
			}
		}

		// Default is to tee stdin to logio and stdout
		var input io.Reader = io.TeeReader(os.Stdin, os.Stdout)
		// If the input is a file, tee it to logio and stdout
		if filename := ctx.String("file"); filename != "" {
			if file, err := os.Open(filename); err != nil {
				return err
			} else {
				input = io.TeeReader(file, os.Stdout)
				defer file.Close()
			}
			// If the input is a command, eval it using sh and tee stdout+stderr to logio
		} else if command := ctx.String("command"); len(command) > 0 {
			cmd := exec.Command("sh", "-c", command)
			cmd.Stdin = os.Stdin
			stdoutPipe, err := cmd.StdoutPipe()
			if err != nil {
				return err
			}
			stderrPipe, err := cmd.StderrPipe()
			if err != nil {
				return err
			}
			outpipe := io.TeeReader(stdoutPipe, os.Stdout)
			errpipe := io.TeeReader(stderrPipe, os.Stderr)
			input = io.MultiReader(outpipe, errpipe)
			go func() {
				if err := cmd.Run(); err != nil {
					os.Exit(1)
				}
			}()
		}

		// Connect to the Logio server
		conn, err := net.Dial("tcp", config.Address)
		if err != nil {
			return err
		}

		logioConn := &LogioConn{
			Reader: resp.NewReader(conn),
			Writer: resp.NewWriter(conn),
			log:    logger,
		}

		// Authenticate and configure the log stream
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

		logs := bufio.NewScanner(input)
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
	log *internal.Logger
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
	return conn.WriteArray(resp.BulkString("PUB"), resp.Integer(time.Now().UnixNano()), resp.BulkString(line))
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
	var app, proc, host string
	if v, ok := u.Query()["app"]; ok {
		app = v[0]
	} else {
		return nil, errors.New("Must specify 'app' parameter in LOGIO_URL")
	}
	if v, ok := u.Query()["proc"]; ok {
		proc = v[0]
	} else {
		return nil, errors.New("Must specify 'proc' parameter in LOGIO_URL")
	}
	if v, ok := u.Query()["host"]; ok {
		host = v[0]
	} else {
		host, _ = os.Hostname()
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
