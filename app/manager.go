package app

import (
	"strings"
)

type addPathReq struct {
	info  PathInfo
	reply chan error
}

type lookupReq struct {
	host  string
	path  string
	reply chan backend
}

type mappingkey struct {
	host   string
	prefix string
}

var addPathChan chan addPathReq = make(chan addPathReq)
var lookupChan chan lookupReq = make(chan lookupReq)

func AddPath(info PathInfo) {
	reply := make(chan error)
	addPathChan <- addPathReq{info, reply}
	<-reply
}

func LookupBackend(host, path string) backend {
	reply := make(chan backend)
	lookupChan <- lookupReq{host, path, reply}
	return <-reply
}

func lookupPath(mapping map[mappingkey]backend, host, path string) backend {
	var bestPrefix string
	var bestBackend backend
	// Find exact match on host first
	for mapkey, backend := range mapping {
		if mapkey.host == host && strings.HasPrefix(path, mapkey.prefix) {
			if bestBackend == nil || len(bestPrefix) < len(mapkey.prefix) {
				bestPrefix = mapkey.prefix
				bestBackend = backend
			}
		}
	}

	if bestBackend == nil {
		for mapkey, backend := range mapping {
			if mapkey.host == "" && strings.HasPrefix(path, mapkey.prefix) {
				if bestBackend == nil || len(bestPrefix) < len(mapkey.prefix) {
					bestPrefix = mapkey.prefix
					bestBackend = backend
				}
			}
		}
	}

	return bestBackend
}

func backendManager() {
	var mapping map[mappingkey]backend = make(map[mappingkey]backend)
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
			mapping[mappingkey{req.info.Host, req.info.Prefix}] = NewBackend(req.info)
			req.reply <- nil

		case msg := <-lookupChan:
			ret := lookupPath(mapping, msg.host, msg.path)

			if ret == nil && externalLookupUrl != "" {
				externalLookupReq <- msg
			} else {
				msg.reply <- ret
			}

		case msg := <-externalLookupResp:
			// Route replies to the client, while updating our mapping table as a cache
			var backend backend
			if msg.info != nil {
				backend = NewBackend(*msg.info)
				mapping[mappingkey{msg.info.Host, msg.info.Prefix}] = backend
			}
			msg.req.reply <- backend
		}
	}
}
