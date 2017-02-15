package main

import (
	"context"
	"log"
	"net"

	redis "gopkg.in/redis.v5"

	"github.com/kevin-cantwell/logio/internal/server"
)

func main() {
	port := ":7701"

	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	serverAddr, err := net.ResolveTCPAddr("tcp", port)
	if err != nil {
		log.Fatal(err)
	}

	ln, err := net.ListenTCP("tcp", serverAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	log.Println("INFO Listening on tcp", port)

	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			log.Println("ERROR accepting tcp conn:", err)
			continue
		}

		go func(conn *net.TCPConn) {
			addr := conn.RemoteAddr().String()
			log.Printf("INFO Client connected: %s\n", addr)
			defer log.Printf("INFO Client disconnected: %s\n", addr)
			defer conn.Close()

			server.NewClientConn(conn, redisClient).Handle(context.Background())
		}(conn)
	}
}
