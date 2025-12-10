package main

import (
	"fmt"
	"sync"
	"time"
)

type logger struct {
	lines   []string
	limit   int
	updated func()
	mutex   sync.Mutex
}

func newLogger(limit int) *logger {
	// ensure limit
	if limit <= 0 {
		limit = 200
	}

	return &logger{
		limit: limit,
	}
}

func (l *logger) Append(format string, args ...any) {
	// acquire mutex
	l.mutex.Lock()
	defer l.mutex.Unlock()

	// format line
	line := fmt.Sprintf("%s | %s", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))

	// append line and enforce limit
	l.lines = append(l.lines, line)
	if len(l.lines) > l.limit {
		l.lines = l.lines[len(l.lines)-l.limit:]
	}

	// call update callback
	if l.updated != nil {
		go l.updated()
	}
}

func (l *logger) Snapshot() []string {
	// acquire mutex
	l.mutex.Lock()
	defer l.mutex.Unlock()

	// copy lines
	lines := make([]string, len(l.lines))
	copy(lines, l.lines)

	return lines
}

func (l *logger) Bind(fn func()) {
	// acquire mutex
	l.mutex.Lock()
	defer l.mutex.Unlock()

	// set callback
	l.updated = fn
}
