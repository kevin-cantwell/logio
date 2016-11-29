package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
)

func main() {
	// The raw log messages that clients send over UDP to the logio server
	incomingLogs := make(chan LogMessage, 1000)
	defer close(incomingLogs)

	go ListenUDP(incomingLogs, ":7514")
	ListenHTTP(incomingLogs, ":7575")
}

func ListenHTTP(incomingLogs <-chan LogMessage, port string) {
	// Fan out log messages to all connected clients
	clients := map[*http.Request]chan LogMessage{}
	var mu sync.RWMutex
	go func() {
		for incomingLog := range incomingLogs {
			mu.RLock()
			for r, outgoingLogs := range clients {
				select {
				case outgoingLogs <- incomingLog:
				default:
					fmt.Println(r.Host, "Dropped log")
				}
			}
			mu.RUnlock()
		}
	}()

	http.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.Host, "Connection opened")
		defer fmt.Println(r.Host, "Connection closed")

		// Register this connection to receive incoming logs via fan-out above
		incomingLogs := make(chan LogMessage, 1000)
		mu.Lock()
		clients[r] = incomingLogs
		mu.Unlock()

		// Make sure we close channel when the client closes the connection
		go func() {
			notif, ok := w.(http.CloseNotifier)
			if ok {
				<-notif.CloseNotify()
				fmt.Println(r.Host, "CloseNotify")

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
		for logmsg := range incomingLogs {
			name := logmsg.Name
			if name == "" {
				name = logmsg.IP
			}
			fmt.Fprintf(w, "[%s] %s", name, logmsg.Log)
			flusher.Flush()
		}
	})

	fmt.Println("Listening on tcp", port)
	http.ListenAndServe(port, nil)
}

func ListenUDP(incomingLogs chan<- LogMessage, port string) {
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

		logmsg := LogMessage{
			IP: addr.IP.String(),
		}
		if err := json.Unmarshal(buf[0:n], &logmsg); err != nil {
			fmt.Println("Error:", err)
			continue
		}
		incomingLogs <- logmsg
	}
}

type LogMessage struct {
	Log      string   `json:"log"`
	IP       string   `json:"ip"`
	Name     string   `json:"name,omitempty"`
	Includes []string `json:"includes,omitempty"`
	Excludes []string `json:"excludes,omitempty"`
}

// var (
// 	tails = map[*Tail]bool{}
// 	mu    sync.Mutex
// )

// type Tail struct {
// 	messages chan LogMessage
// 	filter   Filter
// }

// func (tail *Tail) Send(logmsg LogMessage) error {
// 	// if tail.filter, then:
// 	select {
// 	case tail.messages <- logmsg:
// 		return nil
// 	default:
// 		return errors.New("error: tail unavailable")
// 	}
// }

// func (tail *Tail) Wants(logmsg LogMessage) bool {
// 	// TODO: Filter tails based on log content
// 	return true
// }

// type Filter struct {
// }

// type Log struct {
// 	Time int64
// 	Msg  string
// }

// type Logs struct {
// 	a []Log
// }

// func (incomingLogs *Logs) Append(l Log) {
// 	incomingLogs.a = append(incomingLogs.a, l)
// }

// func (incomingLogs *Logs) Before(upper int64) Logs {
// 	u := sort.Search(len(incomingLogs.a), func(i int) bool {
// 		return incomingLogs.a[i].Time >= upper
// 	})
// 	return Logs{
// 		// u+1 gives us a between function that is inclusive of the upper bound
// 		a: incomingLogs.a[:u+1],
// 	}
// }

// func (incomingLogs *Logs) Before(upper int64) Logs {
// 	u := sort.Search(len(incomingLogs.a), func(i int) bool {
// 		return incomingLogs.a[i].Time >= upper
// 	})
// 	return Logs{
// 		// u+1 gives us a between function that is inclusive of the upper bound
// 		a: incomingLogs.a[:u+1],
// 	}
// }

// func (incomingLogs *Logs) Between(lower, upper int64) Logs {
// 	l := sort.Search(len(incomingLogs.a), func(i int) bool {
// 		return incomingLogs.a[i].Time >= lower
// 	})
// 	u := sort.Search(len(incomingLogs.a), func(i int) bool {
// 		return incomingLogs.a[i].Time >= upper
// 	})
// 	return Logs{
// 		// u+1 gives us a between function that is inclusive of the upper bound
// 		a: incomingLogs.a[l : u+1],
// 	}
// }
