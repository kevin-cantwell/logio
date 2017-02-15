package server

import (
	"log"
	"os"
)

var (
	Debug = (os.Getenv("LOG_LEVEL") == "DEBUG")
)

type Logger struct {
	*log.Logger
	ID string
}

func (l *Logger) INFO(args ...interface{}) {
	l.Println(append([]interface{}{"INFO", l.ID}, args...)...)
}

func (l *Logger) INFOf(format string, args ...interface{}) {
	l.Printf(format+"\n", append([]interface{}{"INFO", l.ID}, args...)...)
}

func (l *Logger) ERROR(args ...interface{}) {
	l.Println(append([]interface{}{"ERROR", l.ID}, args...)...)
}

func (l *Logger) ERRORf(format string, args ...interface{}) {
	l.Printf(format+"\n", append([]interface{}{"ERROR", l.ID}, args...)...)
}

func (l *Logger) DEBUG(args ...interface{}) {
	if Debug {
		l.Println(append([]interface{}{"DEBUG", l.ID}, args...)...)
	}
}

func (l *Logger) DEBUGf(format string, args ...interface{}) {
	if Debug {
		l.Printf(format+"\n", append([]interface{}{"DEBUG", l.ID}, args...)...)
	}
}

func (l *Logger) IFERR(msg string, args ...interface{}) {
	for _, arg := range args {
		if err, ok := arg.(error); ok {
			l.ERROR(msg, err)
		}
	}
}
