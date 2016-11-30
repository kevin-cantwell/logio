package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/kevin-cantwell/logio"
)

func main() {
	// The raw log messages that clients send over UDP to the logio server
	incomingLogs := make(chan logio.LogMessage, 1000)
	defer close(incomingLogs)

	go ListenUDP(incomingLogs, ":7514")
	ListenHTTP(incomingLogs, ":7575")
}

func ListenHTTP(incomingLogs <-chan logio.LogMessage, port string) {
	// Fan out log messages to all connected clients
	clients := map[*http.Request]chan logio.LogMessage{}
	var mu sync.RWMutex
	go func() {
		for incomingLog := range incomingLogs {
			mu.RLock()
			for r, outgoingLogs := range clients {
				select {
				case outgoingLogs <- incomingLog:
				default:
					ip := r.Header.Get("X-Forwarded-For")
					if ip == "" {
						ip = r.RemoteAddr
					}
					fmt.Println(ip, "Dropped log")
				}
			}
			mu.RUnlock()
		}
	}()

	http.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}

		fmt.Println(ip, "Connection opened")
		defer fmt.Println(ip, "Connection closed")

		// Register this connection to receive incoming logs via fan-out above
		incomingLogs := make(chan logio.LogMessage, 1000)
		mu.Lock()
		clients[r] = incomingLogs
		mu.Unlock()

		// Make sure we close channel when the client closes the connection
		go func() {
			notif, ok := w.(http.CloseNotifier)
			if ok {
				<-notif.CloseNotify()
				fmt.Println(ip, "CloseNotify")

				mu.Lock()
				close(incomingLogs)
				delete(clients, r)
				mu.Unlock()
			}
		}()

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
		for msg := range incomingLogs {
			t := msg.Time.UTC().Format("2006-01-02T15:04:05.000")
			fmt.Fprintf(w, "%s %s %s[%s]: %s", t, msg.IP, msg.Procname, msg.Hostname, msg.Log)
			flusher.Flush()
		}
	})

	fmt.Println("Listening on tcp", port)
	http.ListenAndServe(port, nil)
}

func ListenUDP(incomingLogs chan<- logio.LogMessage, port string) {
	/* Lets prepare a address at any address at port 7514*/
	serverAddr, err := net.ResolveUDPAddr("udp", port)
	if err != nil {
		log.Fatal(err)
	}

	/* Now listen at selected port */
	serverConn, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer serverConn.Close()

	buf := make([]byte, 1024)

	fmt.Println("Listening on udp", port)
	for {
		n, addr, err := serverConn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		logmsg := logio.LogMessage{
			IP: addr.IP.String(),
		}
		if err := json.Unmarshal(buf[0:n], &logmsg); err != nil {
			fmt.Println("Error:", err)
			continue
		}
		incomingLogs <- logmsg
	}
}
