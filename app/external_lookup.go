package app

import (
	"github.com/Sirupsen/logrus"
	"github.com/franela/goreq"

	"net/url"
	"time"
)

type externalLookupResp struct {
	req  lookupReq
	info *PathInfo
}

func doLookup(host string, path string) *PathInfo {
	log := logrus.New().WithFields(logrus.Fields{
		"type": "external_lookup",
		"host": host,
		"path": path,
	})
	log.Logger = logrus.StandardLogger()

	log.Info("Asking about pathinfo")
	uri := externalLookupUrl + "?host=" + url.QueryEscape(host) + "&path=" + url.QueryEscape(path)
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

	log.Info("Failed to get pathinfo")
	return nil
}

func externalLookupWorker(jobs <-chan lookupReq, results chan<- externalLookupResp) {
	for j := range jobs {
		results <- externalLookupResp{j, doLookup(j.host, j.path)}
	}
}
