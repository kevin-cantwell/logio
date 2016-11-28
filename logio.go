package logio

import (
	"io"
	"log"
	"net"
	"os"
)

func init() {
	serverAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:7514")
	if err != nil {
		panic(err)
	}

	localAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	conn, err := net.DialUDP("udp", localAddr, serverAddr)
	if err != nil {
		panic(err)
	}

	multi := io.MultiWriter(os.Stderr, conn)

	log.SetOutput(multi)
}
