package app

import "strings"


type addPathReq struct {
	path  PathInfo
	reply chan error
}

type lookupReq struct {
	path  string
	reply chan *PathInfo
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

func pathManager() {
	var mapping []PathInfo = make([]PathInfo, 0)

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
			msg.reply <- ret
		}
	}
}
