package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/kevin-cantwell/logio"
	"github.com/kevin-cantwell/logio/internal/server"
)

func main() {
	brokers := &UserBrokers{
		b: map[string]*server.Broker{
			"TBD": &server.Broker{},
		},
	}

	go ListenTCP(brokers, ":12157") // L=12, O=15, G=7
	ListenHTTP(brokers, ":7575")
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

func ListenHTTP(brokers *UserBrokers, port string) {
	http.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		topics := server.TopicMatcher{
			AppPattern:  r.URL.Query().Get("app"),
			ProcPattern: r.URL.Query().Get("proc"),
		}

		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}

		host, _, err := net.SplitHostPort(ip)
		if err != nil {
			http.Error(w, "Cannot determine ip", http.StatusBadRequest)
			return
		}

		fmt.Printf("SUB hello: %s %s[%s]\n", host, topics.AppPattern, topics.ProcPattern)
		defer fmt.Printf("SUB goodbye: %s %s[%s]\n", host, topics.AppPattern, topics.ProcPattern)

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
			t := msg.Time.UTC().Format("2006-01-02T15:04:05.000")
			fmt.Fprintf(w, "%s %s %s[%s]: %s", t, msg.Host, msg.App, msg.Proc, msg.Log)
			flusher.Flush()
		}
	})

	fmt.Println("Listening on tcp", port)
	http.ListenAndServe(port, nil)
}

// Listens for log publishers and registers them for discovery by subscribers
func ListenTCP(brokers *UserBrokers, port string) {
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

	fmt.Println("Listening on tcp", port)

	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		go func(conn *net.TCPConn) {
			defer conn.Close()

			host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
			if err != nil {
				conn.Write([]byte(err.Error()))
				return
			}

			dec := json.NewDecoder(conn)

			// The header contains the meta-data for the process as well as authentication strings.
			// Every new connection is required to send the header as the first line in the stream.
			var header logio.Header
			if err := dec.Decode(&header); err != nil {
				conn.Write([]byte(err.Error()))
				return
			}

			proc := fmt.Sprintf("%s.%s", header.ProcName, header.ProcID)
			if header.ProcName == "" {
				proc = header.ProcID
			}
			if header.ProcID == "" {
				proc = header.ProcName
			}

			topic := server.Topic{
				App:  header.App,
				Proc: proc,
			}

			fmt.Printf("PUB hello: %s %s[%s]\n", host, topic.App, topic.Proc)
			defer fmt.Printf("PUB goodbye: %s %s[%s]\n", host, topic.App, topic.Proc)

			// TODO: Detect username from connection
			broker := brokers.Get("TBD")

			var msg logio.Message
			for {
				if err := dec.Decode(&msg); err != nil {
					conn.Write([]byte(err.Error()))
					return
				}
				broker.Notify(server.Message{
					Topic: topic,
					Time:  msg.Time,
					Host:  host,
					Log:   msg.Log,
				})
			}
		}(conn)
	}
}

/*------------------------------------ UDP LOGIC --------------------------------------*/

// func ListenUDP(incomingLogs chan<- logio.Message, port string) {
// 	/* Lets prepare a address at any address at port 7514*/
// 	serverAddr, err := net.ResolveUDPAddr("udp", port)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	/* Now listen at selected port */
// 	serverConn, err := net.ListenUDP("udp", serverAddr)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer serverConn.Close()

// 	buf := make([]byte, 1024)

// 	fmt.Println("Listening on udp", port)
// 	for {
// 		n, addr, err := serverConn.ReadFromUDP(buf)
// 		if err != nil {
// 			fmt.Println("Error:", err)
// 			continue
// 		}

// 		logmsg := logio.Message{
// 			IP: addr.IP.String(),
// 		}
// 		if err := json.Unmarshal(buf[0:n], &logmsg); err != nil {
// 			fmt.Println("Error:", err)
// 			continue
// 		}
// 		incomingLogs <- logmsg
// 	}
// }
