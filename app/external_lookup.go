package app

import (
	"github.com/franela/goreq"
	"log"
	"net/url"
	"time"
)

type externalLookupResp struct {
	req  lookupReq
	info *PathInfo
}

func doLookup(host string, path string) *PathInfo {
	uri := externalLookupUrl + "?host=" + url.QueryEscape(host) + "&path=" + url.QueryEscape(path)
	log.Printf("Asking %s about pathinfo\n", uri)
	req := goreq.Request{
		Uri:       uri,
		Accept:    "application/json",
		UserAgent: "Undergang/1.0",
		Timeout:   5 * time.Second,
	}

	if ret, err := req.Do(); err == nil && ret.StatusCode == 200 {
		var path PathInfo
		ret.Body.FromJsonTo(&path)
		return &path
	}
	return nil
}

func externalLookupWorker(jobs <-chan lookupReq, results chan<- externalLookupResp) {
	for j := range jobs {
		results <- externalLookupResp{j, doLookup(j.host, j.path)}
	}
}
