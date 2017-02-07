package main

import (
	"io"
	"log"
	"os"

	"github.com/kevin-cantwell/logio"
)

func main() {
	var in io.Reader = os.Stdin
	if out := logio.Output(); out != nil {
		in = io.TeeReader(os.Stdin, out)
	}
	for {
		if _, err := io.Copy(os.Stdout, in); err != nil && err != io.EOF {
			log.Println("logio: ", err)
		}
	}
}
