### Logio

Logio is a system for publishings log streams to a server and fanning them out to client subscriptions. It consists of a server and simple PUB/SUB client implementations. Configured apps publish logs to a topic via a TCP stream and clients subscribe to one or more topics for a live stream of logs. The result is similar experience to Heroku's log plexing service, only self-hosted.

### Usage

#### Server

Spinning up a Logio server is as simple as the following:

```
$ go get github.com/kevin-cantwell/logio
$ go build -o logio github.com/kevin-cantwell/logio/cmd/logio-server/main.go
$ logio-server
Listening for subscribers on tcp :7702
Listening for publishers on tcp :7701
```

#### Publisher

A publisher is any app that produces logs. This project includes a Go package that will re-configure the default logger to publish logs to a server. Setting it up is as simple as importing the package and configuring a single connection string:

```go
import _ "github.com/kevin-cantwell/logio"
```

```
$ export LOGIO_URL='logio://logio-server.example.com:7701?app=myapp&proc=web&host=foobar'
```

If you are familiar with Redis the connection string above will makes sense to you:
* `logio` is the schema and indicates that this is a Logio server we're connecting to
* `logio-server.example.com` is the domain where the Logio server is hosted.
* `7701` is the default port which handles log publications
* `app`, `proc`, and `host` define the topic to which logs are published. Only `app` is required. The `proc` and `host` values default to the name of the running process and the hostname, respecively. Usually only `app` and `proc` are specified.

#### Subscriber

A subscriber is any client that requests logs on one or more topics. A topic is composed of `app`, `proc`, and `host`. Subscribers may specify regex patterns that match multiple topics, thereby stitching multiple log streams into one (fan-out pattern).

The below subscription matches any apps named `myapp` with a process named either `web` or `worker`:

```
$ curl -N 'http://logio-server.example.com:7702?app=myapp&proc=web|worker'
2016-12-06T22:14:21.448 myapp[worker] i-10-45-22-14: 2016/12/06 17:14:21 INFO This is a worker log
2016-12-06T22:14:21.567 myapp[web] i-10-34-20-12: 2016/12/06 17:14:21 ERROR This is a web error
2016-12-06T22:14:21.782 myapp[web] i-10-34-20-12: 2016/12/06 17:14:21 INFO This is a web log
2016-12-06T22:14:21.901 myapp[web] i-10-34-20-12: 2016/12/06 17:14:21 INFO This is a web log
```

The output of a log subscription is formatted like so:

`<timestamp> <app>[<proc>] <host>: <log>`


