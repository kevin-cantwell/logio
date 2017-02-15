package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/kevin-cantwell/resp"
	redis "gopkg.in/redis.v5"
)

type ClientConn struct {
	*resp.Reader
	*resp.Writer

	conn     *net.TCPConn
	redis    *redis.Client
	log      *Logger
	username string
	app      string
	proc     string
	host     string
}

func NewClientConn(conn *net.TCPConn, r *redis.Client) *ClientConn {
	return &ClientConn{
		Reader: resp.NewReader(conn),
		Writer: resp.NewWriter(conn),
		conn:   conn,
		redis:  r,
		log: &Logger{
			Logger: log.New(os.Stdout, "", log.LstdFlags),
			ID:     fmt.Sprintf("[%s]", conn.RemoteAddr().String()),
		},
	}
}

func (conn *ClientConn) Handle(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // subscriptions need to be notified of done-ness

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		data, err := conn.ReadData()
		if err != nil {
			if err == io.EOF {
				return
			}
			conn.log.IFERR(
				"Write error '"+err.Error()+"':",
				conn.WriteError("ERR "+err.Error()),
			)
			continue
		}

		cmd, ok := data.(resp.Array)
		if !ok {
			conn.log.IFERR(
				"Write error 'ERR all commands must be sent as arrays':",
				conn.WriteError("ERR all commands must be sent as arrays"),
			)
			continue
		}

		if len(cmd) < 1 {
			conn.log.IFERR(
				"Write error 'ERR empty command':",
				conn.WriteError("ERR empty command"),
			)
			continue
		}

		conn.log.DEBUG(cmd.Quote())

		switch strings.ToUpper(cmd[0].Raw()) {
		case "QUIT": // eg: QUIT
			conn.log.IFERR(
				"Failed to handle QUIT command:",
				conn.handleQuit(ctx, cmd),
			)
			return
		case "AUTH": // eg: AUTH <username> <password>
			conn.log.IFERR(
				"Failed to handle AUTH command:",
				conn.handleAuth(ctx, cmd),
			)
		case "APP": // eg: APP <name> <proc> <host>
			conn.log.IFERR(
				"Failed to handle APP command:",
				conn.handleApp(ctx, cmd),
			)
		case "PUB": // PUB <nanoseconds> <log>
			conn.log.IFERR(
				"Failed to handle PUB command:",
				conn.handlePub(ctx, cmd),
			)
		case "SUB": // SUB <app_glob> <proc_glob> <host_glob>
			// Subscriptions stream in the background, so this will return immediately
			conn.log.IFERR(
				"Failed to handle SUB command:",
				conn.handleSub(ctx, cmd),
			)
		default:
			err := fmt.Sprintf("ERR unknown command '%s'", cmd[0].Raw())
			conn.log.IFERR(
				"Write error '"+err+"':",
				conn.WriteError(err),
			)
		}
	}
}

func (conn *ClientConn) handleQuit(ctx context.Context, cmd resp.Array) error {
	return conn.WriteSimpleString("OK")
}

func (conn *ClientConn) handleAuth(ctx context.Context, cmd resp.Array) error {
	if len(cmd) != 3 {
		return conn.WriteError("ERR wrong number of arguments for 'auth' command")
	}

	// TODO: Implement user auth

	conn.username = cmd[1].Raw()

	return conn.WriteSimpleString("OK")
}

func (conn *ClientConn) handleApp(ctx context.Context, cmd resp.Array) error {
	if len(cmd) != 4 {
		return conn.WriteError("ERR wrong number of arguments for 'app' command")
	}
	app := cmd[1].Raw()
	proc := cmd[2].Raw()
	host := cmd[3].Raw()

	if strings.ContainsAny(app, ":\n") || strings.ContainsAny(proc, ":\n") || strings.ContainsAny(host, ":\n") {
		return conn.WriteError("ERR arguments may not contain any ':\\n'")
	}

	conn.app = app
	conn.proc = proc
	conn.host = host

	return conn.WriteSimpleString("OK")
}

// Publishes messages as "<nanos>:<log>". This ensures that duplicate log lines don't write over each other
func (conn *ClientConn) handlePub(ctx context.Context, cmd resp.Array) error {
	if len(cmd) != 3 {
		return conn.WriteError("ERR wrong number of arguments for 'pub' command")
	}

	ts := cmd[1].Raw()
	msg := cmd[2].Raw()

	tsfloat, err := strconv.ParseFloat(ts, 64)
	if err != nil {
		return conn.WriteError("ERR timestamp must be a nanosecond integer")
	}

	key := fmt.Sprintf("%s:%s:%s:%s", conn.username, conn.app, conn.proc, conn.host)
	value := ts + ":" + msg
	if _, err := conn.redis.Publish(key, value).Result(); err != nil {
		// Don't die here. Just log it and drive on. Published logs are real-time only
		conn.log.ERRORf("Failed to publish log at '%s': %v\n", ts, err)
		conn.log.IFERR(
			"Write error 'ERR stream':",
			conn.WriteError("ERR stream: "+ts+":"+msg),
		)
	}

	if _, err := conn.redis.ZAdd(key, redis.Z{Score: tsfloat, Member: value}).Result(); err != nil {
		conn.log.IFERR(
			"Write error 'ERR storage':",
			conn.WriteError("ERR storage: "+ts+":"+msg),
		)
		return err
	}
	return nil
}

// SUB <app_glob> <proc_glob> <host_glob>
// Streams logs as a RESP array:
// <integer:nanos> <simple_string:app> <simple_string:proc> <simple_string:host> <bulk_string:log>
func (conn *ClientConn) handleSub(ctx context.Context, cmd resp.Array) error {
	if len(cmd) != 4 {
		return conn.WriteError("ERR wrong number of arguments for 'sub' command")
	}
	appGlob := cmd[1].Raw()
	procGlob := cmd[2].Raw()
	hostGlob := cmd[3].Raw()

	pattern := fmt.Sprintf("%s:%s:%s:%s", conn.username, appGlob, procGlob, hostGlob)
	pubsub, err := conn.redis.PSubscribe(pattern)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			msg, err := pubsub.ReceiveMessage()
			if err != nil {
				conn.log.ERRORf("PSUBSCRIBE failed on message receipt for '%s': %v", pattern, err)
				conn.log.IFERR(
					"Write error 'ERR internal server error':",
					conn.WriteError("ERR internal server error"),
				)
				return
			}

			channel := strings.Split(msg.Channel, ":")
			if len(channel) != 4 {
				conn.log.ERRORf("Subscribed to invalid channel '%s'", msg.Channel)
				continue
			}

			app := channel[1]
			proc := channel[2]
			host := channel[3]

			payload := strings.SplitN(msg.Payload, ":", 2)
			ts, err := strconv.ParseInt(payload[0], 10, 64)
			if err != nil {
				conn.log.ERRORf("Failed to parse timestamp '%s': %v", payload[0], err)
				continue
			}

			err = conn.WriteArray(
				resp.Integer(ts),            // ts in nanos
				resp.SimpleString(app),      //
				resp.SimpleString(proc),     //
				resp.SimpleString(host),     //
				resp.BulkString(payload[1]), // log
			)
			if err != nil {
				conn.log.ERRORf("Write error '%v':", err)
				return
			}
		}
	}()

	return nil
}
