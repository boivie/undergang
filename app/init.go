package app

import (
	"io"
	"log"
	"net/http"
	"runtime"
)

var externalLookupURL string
var proxyCommand string

func dumpHandler(w http.ResponseWriter, req *http.Request) {
	buf := make([]byte, 1<<20)
	runtime.Stack(buf, true)
	log.Printf("=== received SIGQUIT ===\n*** goroutine dump...\n%s\n*** end\n", buf)
}

func healthHandler(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "OK\n")
}

// Init initializes the application
func Init(extPathLookupURL string, proxyCmd string) {
	proxyCommand = proxyCmd
	externalLookupURL = extPathLookupURL
	go backendManager()

	http.HandleFunc("/__ug__dump", dumpHandler)
	http.HandleFunc("/__ug__health", healthHandler)
	http.HandleFunc("/", forward)
}
