package app
import (
	"strings"
	"github.com/franela/goreq"
	"time"
	"log"
)

type accessLookupReq struct {
	token string
	path  string
	reply chan <- bool
}

type externalAcessLookupResp struct {
	req    accessLookupReq
	prefix string
}

var accessLookupChan chan accessLookupReq = make(chan accessLookupReq)

func LookupAccess(token string, path string) bool {
	reply := make(chan bool)
	accessLookupChan <- accessLookupReq{token, path, reply}
	return <-reply
}

func accessLookupWorker(url string, jobs <- chan accessLookupReq, results chan <- externalAcessLookupResp) {
	for j := range jobs {
		log.Printf("Asking access backend for '%s' to '%s'\n", j.token, j.path)
		req := goreq.Request{
			Uri:         url + "?token=" + j.token + "&path=" + j.path,
			Accept:      "application/json",
			UserAgent:   "Undergang/1.0",
			Timeout:     5 * time.Second,
		}

		var parsed struct {
			Prefix string `json:"string"`
		}
		if ret, err := req.Do(); err == nil && ret.StatusCode == 200 {
			ret.Body.FromJsonTo(&parsed)
		}
		results <- externalAcessLookupResp{j, parsed.Prefix}
	}
}

func accessManager(lookupUrl string) {
	cache := make(map[string]map[string]bool)
	externalReq := make(chan accessLookupReq, 100)
	externalResp := make(chan externalAcessLookupResp, 100)

	for w := 1; w<= 5; w++ {
		go accessLookupWorker(lookupUrl, externalReq, externalResp)
	}

	for {
		select {
		case msg := <-accessLookupChan:
			if lookupUrl == "" {
				// Not configured - grant access to all.
				msg.reply <- true
			} else {
				result := false
				if prefixes, ok := cache[msg.token]; ok {
					for prefix, _ := range prefixes {
						if strings.HasPrefix(msg.path, prefix) {
							result = true
						}
					}
				}

				if result {
					msg.reply <- true
				} else {
					externalReq <- msg
				}
			}
		case msg := <-externalResp:
		// Route replies to the client, while updating our cache
			if msg.prefix != "" {
				if _, ok := cache[msg.req.token]; !ok {
					cache[msg.req.token] = make(map[string]bool)
				}
				cache[msg.req.token][msg.prefix] = true
			}
			msg.req.reply <- msg.prefix != ""
		}
	}
}
