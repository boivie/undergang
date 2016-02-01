package app

import (
	"strings"
)

type addPathReq struct {
	path  PathInfo
	reply chan error
}

type lookupReq struct {
	path  string
	reply chan backend
}

var addPathChan chan addPathReq = make(chan addPathReq)
var lookupChan chan lookupReq = make(chan lookupReq)

func AddPath(path PathInfo) {
	reply := make(chan error)
	addPathChan <- addPathReq{path, reply}
	<-reply
}

func LookupBackend(path string) backend {
	reply := make(chan backend)
	lookupChan <- lookupReq{path, reply}
	return <-reply
}

func lookupPath(mapping map[string]backend, path string) backend {
	var bestPrefix string
	var bestBackend backend
	for prefix, backend := range mapping {
		if strings.HasPrefix(path, prefix) {
			if bestBackend == nil || len(bestPrefix) < len(prefix) {
				bestPrefix = prefix
				bestBackend = backend
			}
		}
	}
	return bestBackend
}

func backendManager() {
	var mapping map[string]backend = make(map[string]backend)
	externalLookupReq := make(chan lookupReq, 100)
	externalLookupResp := make(chan externalLookupResp, 100)

	if externalLookupUrl != "" {
		for w := 1; w <= 5; w++ {
			go externalLookupWorker(externalLookupReq, externalLookupResp)
		}
	}

	for {
		select {
		case req := <-addPathChan:
			mapping[req.path.Prefix] = NewBackend(req.path)
			req.reply <- nil

		case msg := <-lookupChan:
			ret := lookupPath(mapping, msg.path)

			if ret == nil && externalLookupUrl != "" {
				externalLookupReq <- msg
			} else {
				msg.reply <- ret
			}

		case msg := <-externalLookupResp:
		// Route replies to the client, while updating our mapping table as a cache
			var backend backend
			if msg.path != nil {
				backend = NewBackend(*msg.path)
				mapping[msg.path.Prefix] = backend
			}
			msg.req.reply <- backend
		}
	}
}
