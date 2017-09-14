package app

import (
	"strings"

	"github.com/Sirupsen/logrus"
)

type addPathReq struct {
	info  PathInfo
	reply chan error
}

type lookupReq struct {
	host  string
	path  string
	reply chan Backend
}

type unregisterReq struct {
	id int
}

type mappingkey struct {
	host   string
	prefix string
}

var addPathChan = make(chan addPathReq)
var lookupChan = make(chan lookupReq)
var unregisterChan = make(chan unregisterReq)

// AddPath adds a backend to the manager
func AddPath(info PathInfo) {
	reply := make(chan error)
	addPathChan <- addPathReq{info, reply}
	<-reply
}

// LookupBackend looks up a backend given a host and path
func LookupBackend(host, path string) Backend {
	reply := make(chan Backend)
	lookupChan <- lookupReq{host, path, reply}
	return <-reply
}

// UnregisterBackend unregisters a backend from the manager
func UnregisterBackend(id int) {
	unregisterChan <- unregisterReq{id}
}

func lookupPath(mapping map[mappingkey]Backend, host, path string) Backend {
	var bestPrefix string
	var bestBackend Backend
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
	log := logrus.New().WithFields(logrus.Fields{
		"type": "manager",
	})
	log.Logger = logrus.StandardLogger()

	mapping := make(map[mappingkey]Backend)
	externalLookupReq := make(chan lookupReq, 100)
	externalLookupResp := make(chan externalLookupResp, 100)

	if externalLookupURL != "" {
		for w := 1; w <= 5; w++ {
			go externalLookupWorker(externalLookupReq, externalLookupResp)
		}
	}

	var currentID = 0

	addBackend := func(info PathInfo) Backend {
		key := mappingkey{info.Host, info.Prefix}
		// There can be a race when we have multiple external look-ups ongoing
		if existing, ok := mapping[key]; ok {
			return existing
		}

		currentID++
		backend := NewBackend(currentID, info)
		log.Infof("Adding backend %d -> '%s%s'", currentID, info.Host, info.Prefix)
		mapping[key] = backend
		return backend
	}

	for {
		select {
		case req := <-addPathChan:
			addBackend(req.info)
			req.reply <- nil

		case msg := <-lookupChan:
			ret := lookupPath(mapping, msg.host, msg.path)

			if ret == nil && externalLookupURL != "" {
				externalLookupReq <- msg
			} else {
				if ret != nil {
					ret.Start()
				}
				msg.reply <- ret
			}

		case msg := <-externalLookupResp:
			// Route replies to the client, while updating our mapping table as a cache
			var backend Backend
			if msg.info != nil {
				backend = addBackend(*msg.info)
				backend.Start()
			}
			msg.req.reply <- backend

		case req := <-unregisterChan:
			for mapkey, backend := range mapping {
				if backend.ID() == req.id {
					log.Infof("Removing backend %d -> '%s%s'", req.id, backend.GetInfo().Host, backend.GetInfo().Prefix)
					delete(mapping, mapkey)
				}
			}
		}
	}
}
