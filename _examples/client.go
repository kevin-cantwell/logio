package main

import (
	"log"
	"os"
	"time"

	"github.com/kevin-cantwell/logio"
)

func main() {
	logio.Debug = true
	for {
		log.Println(os.Args[1])
		time.Sleep(time.Millisecond * 100)
	}
}
