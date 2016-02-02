package app
import (
	"github.com/franela/goreq"
	"time"
)

type externalLookupResp struct {
	req  lookupReq
	info *PathInfo
}

func doLookup(host string, path string) *PathInfo {
	req := goreq.Request{
		Uri:         externalLookupUrl + "?host=" + host + "&path=" + path,
		Accept:      "application/json",
		UserAgent:   "Undergang/1.0",
		Timeout:     5 * time.Second,
	}

	if ret, err := req.Do(); err == nil && ret.StatusCode == 200 {
		var path PathInfo
		ret.Body.FromJsonTo(&path)
		return &path
	}
	return nil
}

func externalLookupWorker(jobs <- chan lookupReq, results chan <- externalLookupResp) {
	for j := range jobs {
		results <- externalLookupResp{j, doLookup(j.host, j.path)}
	}
}
