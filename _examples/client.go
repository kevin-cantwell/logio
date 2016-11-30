package main

import (
	"log"
	"os"
	"time"

	"github.com/kevin-cantwell/logio"
)

func main() {
	logio.Debug = true
	logio.Tags = map[string]string{
		"arg1": os.Args[1],
	}
	for {
		log.Println(os.Args)
		time.Sleep(time.Millisecond * 10)
	}
}
