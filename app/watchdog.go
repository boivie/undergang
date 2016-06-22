package app

import (
	"fmt"
	"runtime"
	"time"
)

func watchdog(who interface{}) chan chan bool {
	reqs := make(chan chan bool, 1)
	pc := make([]uintptr, 11)
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	file, line := f.FileLine(pc[0])

	m := func() {
		for {
			<-time.After(10 * time.Second)
			reply := make(chan bool)
			select {
			case reqs <- reply:
			// pass
			default:
				panic(fmt.Sprintf("Failed to send watchdog message from %s:%d, %v", file, line, who))
			}
			select {
			case <-reply:
			// pass
			case <-time.After(1 * time.Second):
				panic(fmt.Sprintf("Failed to receive reply from %s:%d, %v", file, line, who))
			}
		}
	}

	go m()
	return reqs
}
