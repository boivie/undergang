package app

import (
	"bytes"
	"log"
	"runtime"
	"strconv"
	"time"
)

func watchdog(who interface{}) chan chan bool {
	reqs := make(chan chan bool, 1)
	pc := make([]uintptr, 11)
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	file, line := f.FileLine(pc[0])

	goid := getGID()

	m := func() {
		for {
			<-time.After(10 * time.Second)
			reply := make(chan bool)
			select {
			case reqs <- reply:
			// pass
			default:
				log.Panicf("Failed to send watchdog message to goroutine %d, %s:%d, %v", goid, file, line, who)
			}
			select {
			case <-reply:
			// pass
			case <-time.After(10 * time.Second):
				log.Panicf("Failed to receive reply from goroutine %d, %s:%d, %v", goid, file, line, who)
			}
		}
	}

	go m()
	return reqs
}

// Hacky way of getting a goroutine ID.
func getGID() uint64 {
	b := make([]byte, 64)
	return extractGID(b[:runtime.Stack(b, false)])
}

func extractGID(stack []byte) uint64 {
	b := bytes.TrimPrefix(stack, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	gid, _ := strconv.ParseUint(string(b), 10, 64)
	return gid
}
