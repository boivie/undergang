package app

import (
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/franela/goreq"
)

const serverAuthEndpoint = "/__undergang_02648018bfd74fa5a4ed50db9bb07859_auth"
const serverAuthCookie = "undergang_02648018bfd74fa5a4ed50db9bb07859_auth"
const serverAuthDuration = 24 * 3600

func getCookieToken(info PathInfo) string {
	return base64URLEncode([]byte(info.Host + "/" + info.Prefix))
}

func serveValidateServerAuth(backend Backend, w http.ResponseWriter, req *http.Request) bool {
	log := backend.GetLogger().WithField("type", "server_auth")
	info := backend.GetInfo()
	serverAuth := info.ServerAuth

	if serverAuth == nil {
		return false
	}

	if !strings.HasSuffix(req.URL.Path, serverAuthEndpoint) {
		return false
	}

	originalPath := strings.Replace(req.URL.Path, serverAuthEndpoint, "", 1)

	if code := req.URL.Query().Get("code"); code != "" {
		fmt.Printf("Asking server %s about code %s\n", serverAuth.ValidateURL, code)
		gr := goreq.Request{
			Method:      "POST",
			Uri:         serverAuth.ValidateURL,
			ContentType: "application/x-www-form-urlencoded",
			Accept:      "application/json",
			UserAgent:   "Undergang/" + undergangVersion,
			Body:        "code=" + code + "&host=" + req.Host + "&path=" + originalPath,
			Timeout:     5 * time.Second,
		}

		var parsed struct {
			// Not really used.
			AccessToken string `json:"access_token"`
		}

		if ret, err := gr.Do(); err == nil && ret.StatusCode == 200 {
			if ret.Body.FromJsonTo(&parsed) == nil && parsed.AccessToken != "" {
				cookie := &http.Cookie{
					Path:  info.Prefix,
					Name:  serverAuthCookie,
					Value: NewTimestampSigner(sha1.New()).Sign(getCookieToken(info)),
				}
				http.SetCookie(w, cookie)

				fmt.Println("User authenticated!")
				http.Redirect(w, req, originalPath, 302)
			} else {
				respond(log, w, req, "Authentication server failure", http.StatusForbidden)
			}
		} else {
			respond(log, w, req, "Authentication server denied code", http.StatusForbidden)
		}
	} else {
		respond(log, w, req, "No code provided", http.StatusForbidden)
	}

	return true
}

func getScheme(r *http.Request) string {
	if r.URL.Scheme == "https" {
		return "https"
	}
	if strings.HasPrefix(r.Proto, "HTTPS") {
		return "https"
	}
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		return "https"
	}
	return "http"
}

func serveServerAuth(backend Backend, w http.ResponseWriter, req *http.Request) bool {
	serverAuth := backend.GetInfo().ServerAuth

	if serverAuth == nil {
		return false
	}

	cookie, err := req.Cookie(serverAuthCookie)
	if err == nil {
		payload, err := NewTimestampSigner(sha1.New()).Verify(cookie.Value, serverAuthDuration)
		if err == nil {
			if payload == getCookieToken(backend.GetInfo()) {
				return false
			}
		}
	}

	uri := getScheme(req) + "://" + req.Host + req.URL.Path + serverAuthEndpoint
	redirect := serverAuth.AuthURL + "?redirect_uri=" + url.QueryEscape(uri)
	http.Redirect(w, req, redirect, 302)
	return true
}
