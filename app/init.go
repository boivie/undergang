package app
import (
	"net/http"
	"runtime"
	"log"
	"io"
)

var externalLookupUrl string
var proxyCommand string

func dumpHandler(w http.ResponseWriter, req *http.Request) {
	buf := make([]byte, 1 << 20)
	runtime.Stack(buf, true)
	log.Printf("=== received SIGQUIT ===\n*** goroutine dump...\n%s\n*** end\n", buf)
}

func healthHandler(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "OK\n")
}

func Init(externalPathLookupUrl_ string, accessLookupUrl string, proxyCommand_ string) {
	proxyCommand = proxyCommand_
	externalLookupUrl = externalPathLookupUrl_
	go backendManager()

	http.HandleFunc("/__ug__dump", dumpHandler)
	http.HandleFunc("/__ug__health", healthHandler)
	http.HandleFunc("/", Forward)
}
