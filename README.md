### Logio

Logio is a fast and simple log multiplexer for streaming real-time (like, "now," now) logs. It also provides basic log storage, retrieval, and navigation.

Think Heroku logs on steriods :)

The Logio system consistes of three statically linked binaries and a database:

1. `logio-server`: A horizontally scalable TCP server that streams, serves, and stores logs.
1. `logio-agent`: An agent that sources logs from a file or process and publishes them to the server.
1. `logs`: A cli tool that subscribes to log streams or fetches stored logs.
1. A Redis server that handles pubsub and storage.

Logio is _not_ a syslog system. It does its best to recover from errors or connection issues and missing logs are highly unlikely, but there is ultimately no guarantee that all logs will make it to the server. The goals here are performance, utility, and cost, while sacrificing perfection.

### Installation

You can either use one of the docker images or install all the binaries from scratch:

`go get -u github.com/kevin-cantwell/logio/...`

### Usage

##### logio-server:

Spinning up a server is as simple as the following:

```
$ export REDIS_URL=redis://localhost:6379 
$ logio-server
```

TCP connections accepted on port 7701.

##### logio-agent:

The agent is responsible for collecting logs and sending them to the server. Any logs it receive will also be piped to stdout (and stderr when available). It must be configured with the server connection info as well as some metadata about the app for which it is collection logs:

```
$ export LOGIO_URL="logio://fooname:foopass@localhost:7701?app=myapp&proc=web" 
$ logio-agent -f /var/logs/web.log
```

-or-

```
$ logio-agent -S localhost -P 7701 -U fooname -W foopass -a myapp -p web -f /var/logs/web.log
```

The agent may also accept log sources from a command or stdin:

```
$ export LOGIO_URL="logio://fooname:foopass@localhost:7701?app=myapp&proc=web" 
$ logio-agent --command "/bin/web"
```

-or-

```
$ export LOGIO_URL="logio://fooname:foopass@localhost:7701?app=myapp&proc=web" 
/bin/web | logio-agent
```

The one drawback of piping output to the agent is that it cannot tell the difference between stdout and stderr, nor will early termination result in a non-zero exit code. If proper exit codes are required, use the `--command, -c` flag.

##### logs cli:

The `logs` cli fetches or streams logs from the server. Log sources can be filtered across app, proc, and host using glob-style patterns:

```
$ export LOGIO_URL="logio://fooname:foopass@localhost:7701" 
logs -a myapp -p w*
```

The output is prefixed with an ISO 8601 timestamp and the source information, followed by the log line itself:

```
2017-02-17T15:26:40.939Z myapp[web] ip-10-10-14-254: 2017/02/17 15:26:40 This web log line
2017-02-17T15:26:41.143Z myapp[worker] ip-10-10-13-144: 2017/02/17 15:26:41 This is a worker log line
```

##### LOGIO_URL:

Logio supports a connection string for configuring servers and clients. It's similar in usage to a Redis connection string and follows a URL structure and is set via the env var `LOGIO_URL`:

`logio://<logio_username>:<logio_password>@<host>:<port>?app=<appname>&proc=<procname>&host=<hostname>`

Note that if `LOGIO_URL` is set, it overrides any command-line flags passed in.

