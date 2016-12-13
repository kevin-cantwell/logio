package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/kevin-cantwell/logio/internal/server"
)

func main() {
	brokers := &UserBrokers{
		b: map[string]*server.Broker{
			"TBD": &server.Broker{},
		},
	}

	go ListenPublishers(brokers, ":7701")
	ListenSubscribers(brokers, ":7702")
}

type UserBrokers struct {
	mu sync.RWMutex
	b  map[string]*server.Broker
}

func (ub *UserBrokers) Get(username string) *server.Broker {
	ub.mu.RLock()
	defer ub.mu.RUnlock()

	return ub.b[username]
}

func (ub *UserBrokers) Set(username string, broker *server.Broker) {
	ub.mu.Lock()
	defer ub.mu.Unlock()

	ub.b[username] = broker
}

func ListenSubscribers(brokers *UserBrokers, port string) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		topics := server.TopicMatcher{
			AppPattern:  r.URL.Query().Get("app"),
			ProcPattern: r.URL.Query().Get("proc"),
			HostPattern: r.URL.Query().Get("host"),
			LogPattern:  r.URL.Query().Get("log"),
		}

		addr := r.Header.Get("X-Forwarded-For")
		if addr == "" {
			addr = r.RemoteAddr
		}

		ip, _, err := net.SplitHostPort(addr)
		if err != nil {
			http.Error(w, "Cannot determine ip", http.StatusBadRequest)
			return
		}

		log.Printf("Subscriber opened: ip=%s app=%s proc=%s host=%s\n", ip, topics.AppPattern, topics.ProcPattern, topics.HostPattern)
		defer log.Printf("Subscriber closed: ip=%s app=%s proc=%s host=%s\n", ip, topics.AppPattern, topics.ProcPattern, topics.HostPattern)

		broker := brokers.Get("TBD")
		subscription := broker.Subscribe(topics)

		// Make sure we unsubscribe when the client closes the connection
		go func(subscription *server.Subscription) {
			notif, ok := w.(http.CloseNotifier)
			if ok {
				<-notif.CloseNotify()
				broker.Unsubscribe(subscription)
			}
		}(subscription)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Begin streaming. Channel will close when CloseNotifier returns
		// E.g.: 2006-01-02T15:04:05.000 <ip> <procname>[<hostname>]: <log>
		for msg := range subscription.Messages() {
			t := msg.Log.Time.UTC().Format("2006-01-02T15:04:05.000")
			fmt.Fprintf(w, "%s %s[%s] %s: %s", t, msg.Topic.App, msg.Topic.Proc, msg.Topic.Host, msg.Log.Raw)
			flusher.Flush()
		}
	})

	fmt.Println("Listening for subscribers on tcp", port)
	http.ListenAndServe(port, nil)
}

// Listens for log publishers and registers them for discovery by subscribers
func ListenPublishers(brokers *UserBrokers, port string) {
	/* Lets prepare a address at any address at port 7514*/
	serverAddr, err := net.ResolveTCPAddr("tcp", port)
	if err != nil {
		log.Fatal(err)
	}

	ln, err := net.ListenTCP("tcp", serverAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	fmt.Println("Listening for publishers on tcp", port)

	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		go func(conn *net.TCPConn) {
			defer conn.Close()

			ip, _, err := net.SplitHostPort(conn.RemoteAddr().String())
			if err != nil {
				conn.Write([]byte(err.Error()))
				return
			}

			dec := json.NewDecoder(conn)

			// Every new connection is required to send the Topic as the first line in the stream.
			var topic server.Topic
			if err := dec.Decode(&topic); err != nil {
				conn.Write([]byte(err.Error()))
				return
			}

			log.Printf("Publisher opened: ip=%s app=%s proc=%s host=%s\n", ip, topic.App, topic.Proc, topic.Host)
			defer log.Printf("Publisher closed: ip=%s app=%s proc=%s host=%s\n", ip, topic.App, topic.Proc, topic.Host)

			// TODO: Authenticate connection and retrieve brokers by username
			broker := brokers.Get("TBD")

			var l server.Log
			for {
				if err := dec.Decode(&l); err != nil {
					conn.Write([]byte(err.Error()))
					return
				}
				broker.Notify(l, topic)
			}
		}(conn)
	}
}
