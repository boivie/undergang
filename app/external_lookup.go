package app

import (
	"net/url"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/franela/goreq"
	"github.com/satori/go.uuid"
)

type externalLookupResp struct {
	req  lookupReq
	info *PathInfo
}

func doLookup(host string, path string) *PathInfo {
	requestID := uuid.NewV4().String()
	log := logrus.New().WithFields(logrus.Fields{
		"type":       "external_lookup",
		"host":       host,
		"path":       path,
		"request_id": requestID,
	})
	log.Logger = logrus.StandardLogger()

	log.Info("Asking about pathinfo")
	uri := externalLookupURL + "?host=" + url.QueryEscape(host) + "&path=" + url.QueryEscape(path)
	req := goreq.Request{
		Uri:       uri,
		Accept:    "application/json",
		UserAgent: "Undergang/" + undergangVersion,
		Timeout:   10 * time.Second,
	}.WithHeader("X-Request-ID", requestID)

	ret, err := req.Do()
	if err != nil {
		log.Warnf("External lookup request failed: %v", err)
		return nil
	} else if ret.StatusCode == 404 {
		log.Infof("External lookup doesn't know about the host and path")
		return nil
	} else if ret.StatusCode != 200 {
		log.Warnf("External lookup request returned unexpected status code %d", ret.StatusCode)
		return nil
	}

	var info PathInfo
	ret.Body.FromJsonTo(&info)
	return &info
}

func externalLookupWorker(jobs <-chan lookupReq, results chan<- externalLookupResp) {
	for j := range jobs {
		results <- externalLookupResp{j, doLookup(j.host, j.path)}
	}
}
