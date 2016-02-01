package app

import (
	"strings"
	"github.com/franela/goreq"
	"time"
)

type addPathReq struct {
	path  PathInfo
	reply chan error
}

type lookupReq struct {
	path  string
	reply chan backend
}

type externalLookupResp struct {
	req  lookupReq
	path *PathInfo
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

func externalLookupWorker(url string, jobs <- chan lookupReq, results chan <- externalLookupResp) {
	for j := range jobs {
		req := goreq.Request{
			Uri:         url + "?path=" + j.path,
			Accept:      "application/json",
			UserAgent:   "Undergang/1.0",
			Timeout:     5 * time.Second,
		}

		if ret, err := req.Do(); err == nil && ret.StatusCode == 200 {
			var path PathInfo
			ret.Body.FromJsonTo(&path)
			results <- externalLookupResp{j, &path}
		} else {
			results <- externalLookupResp{j, nil}
		}
	}
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

func backendManager(externalLookupUrl string) {
	var mapping map[string]backend = make(map[string]backend)
	externalLookupReq := make(chan lookupReq, 100)
	externalLookupResp := make(chan externalLookupResp, 100)

	if externalLookupUrl != "" {
		for w := 1; w <= 5; w++ {
			go externalLookupWorker(externalLookupUrl, externalLookupReq, externalLookupResp)
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
