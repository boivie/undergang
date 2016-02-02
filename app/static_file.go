package app
import (
	"net/http"
	"encoding/base64"
	"strconv"
	"strings"
)


func serveStatic(backend backend, w http.ResponseWriter, req *http.Request) bool {
	if backend.GetInfo().StaticOverrides != nil {
		url := backend.GetInfo().Backend.BasePath + strings.TrimPrefix(req.URL.Path, backend.GetInfo().Prefix)
		if contents, ok := backend.GetInfo().StaticOverrides[url]; ok {
			buf, err := base64.StdEncoding.DecodeString(contents)
			if err == nil {
				w.Header().Add("Content-Length", strconv.Itoa(len(buf)))
				w.Write(buf)
				return true
			}
		}
	}
	return false
}
