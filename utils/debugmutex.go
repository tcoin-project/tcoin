package utils

import (
	"log"
	"runtime"
	"sync"
)

type DebugMutex struct {
	mut sync.Mutex
}

func (m *DebugMutex) Lock() {
	m.mut.Lock()
	_, file, no, ok := runtime.Caller(1)
	log.Printf("lock: %s %d %v", file, no, ok)
}

func (m *DebugMutex) Unlock() {
	_, file, no, ok := runtime.Caller(1)
	log.Printf("unlock: %s %d %v", file, no, ok)
	m.mut.Unlock()
}
