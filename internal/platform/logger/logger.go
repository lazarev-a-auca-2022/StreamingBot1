package logger

import (
	"log"
)

// Logger is a tiny abstraction that can be swapped with a structured logger later.
type Logger struct {
	level string
}

func New(level string) Logger {
	return Logger{level: level}
}

func (l Logger) Info(msg string) {
	log.Printf("level=%s msg=%q", l.level, msg)
}

func (l Logger) Warn(msg string) {
	log.Printf("level=warn msg=%q", msg)
}

func (l Logger) Error(msg string) {
	log.Printf("level=error msg=%q", msg)
}
