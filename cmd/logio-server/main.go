package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"

	redis "gopkg.in/redis.v5"

	"github.com/kevin-cantwell/logio/internal/server"
)

func main() {
	port := ":7701"

	opts, err := parseRedisOptionsURL(os.Getenv("REDIS_URL"))
	redisClient := redis.NewClient(opts)

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

func parseRedisOptionsURL(redisURL string) (*redis.Options, error) {
	if redisURL == "" {
		return &redis.Options{
			Addr:     "localhost:6379",
			Password: "", // no password set
			DB:       0,  // use default DB
		}, nil
	}

	u, err := url.Parse(redisURL)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "redis" {
		return nil, fmt.Errorf("invalid redis URL scheme: %s", u.Scheme)
	}

	h, p, err := net.SplitHostPort(u.Host)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL host: %s", u.Host)
	}

	address := net.JoinHostPort(h, p)

	var password string
	if u.User != nil {
		p, isSet := u.User.Password()
		if isSet {
			password = p
		}
	}

	var db int
	if len(u.Path) > 2 {
		db, _ = strconv.Atoi(u.Path[1:])
		fmt.Println("DB:", db)
	}

	return &redis.Options{
		Addr:     address,
		Password: password,
		DB:       db,
	}, nil
}
