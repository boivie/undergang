package app
import (
	"net/http"
	"encoding/base64"
	"strconv"
)


func serveStatic(mapping map[string]string, w http.ResponseWriter, req *http.Request) bool {
	if mapping != nil {
		if contents, ok := mapping[req.RequestURI]; ok {
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
