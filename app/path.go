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
	reply chan *PathInfo
}

type externalLookupResp struct {
	req  lookupReq
	info *PathInfo
}

var addPathChan chan addPathReq = make(chan addPathReq)
var lookupChan chan lookupReq = make(chan lookupReq)

func AddPath(path PathInfo) {
	reply := make(chan error)
	addPathChan <- addPathReq{path, reply}
	<-reply
}

func LookupPath(path string) *PathInfo {
	reply := make(chan *PathInfo)
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

func pathManager(externalLookupUrl string) {
	var mapping []PathInfo = make([]PathInfo, 0)
	externalLookupReq := make(chan lookupReq, 100)
	externalLookupResp := make(chan externalLookupResp, 100)

	if externalLookupUrl != "" {
		for w := 1; w<= 5; w++ {
			go externalLookupWorker(externalLookupUrl, externalLookupReq, externalLookupResp)
		}
	}

	for {
		select {
		case req := <-addPathChan:
			mapping = append(mapping, req.path)
			req.reply <- nil

		case msg := <-lookupChan:
			var ret *PathInfo
			for _, iter := range mapping {
				if strings.HasPrefix(msg.path, iter.Prefix) {
					if ret == nil ||  len(ret.Prefix) < len(iter.Prefix) {
						ret = &iter
					}
				}
			}
			if ret == nil && externalLookupUrl != "" {
				externalLookupReq <- msg
			} else {
				msg.reply <- ret
			}

		case msg := <-externalLookupResp:
			// Route replies to the client, while updating our mapping table as a cache
			if msg.info != nil {
				mapping = append(mapping, *msg.info)
			}
			msg.req.reply <- msg.info
		}
	}
}
