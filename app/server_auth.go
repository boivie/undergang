package app

import (
	"crypto/sha1"
	"fmt"
	"github.com/franela/goreq"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const SERVER_AUTH_ENDPOINT = "/__undergang_02648018bfd74fa5a4ed50db9bb07859_auth"
const SERVER_AUTH_COOKIE = "undergang_02648018bfd74fa5a4ed50db9bb07859_auth"
const SERVER_AUTH_DURATION = 24 * 3600

func getCookieToken(info PathInfo) string {
	return base64URLEncode([]byte(info.Host + "/" + info.Prefix))
}

func serveValidateServerAuth(backend backend, w http.ResponseWriter, req *http.Request) bool {
	info := backend.GetInfo()
	serverAuth := info.ServerAuth

	if serverAuth == nil {
		return false
	}

	if !strings.HasSuffix(req.URL.Path, SERVER_AUTH_ENDPOINT) {
		return false
	}

	originalPath := strings.Replace(req.URL.Path, SERVER_AUTH_ENDPOINT, "", 1)

	if code := req.URL.Query().Get("code"); code != "" {
		fmt.Printf("Asking server %s about code %s\n", serverAuth.ValidateUrl, code)
		gr := goreq.Request{
			Method:      "POST",
			Uri:         serverAuth.ValidateUrl,
			ContentType: "application/x-www-form-urlencoded",
			Accept:      "application/json",
			UserAgent:   "Undergang/1.0",
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
					Name:  SERVER_AUTH_COOKIE,
					Value: NewTimestampSigner(sha1.New()).Sign(getCookieToken(info)),
				}
				http.SetCookie(w, cookie)

				fmt.Println("User authenticated!")
				http.Redirect(w, req, originalPath, 302)
			} else {
				respond(w, req, "Authentication server failure", http.StatusForbidden)
			}
		} else {
			respond(w, req, "Authentication server denied code", http.StatusForbidden)
		}
	} else {
		respond(w, req, "No code provided", http.StatusForbidden)
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

func serveServerAuth(backend backend, w http.ResponseWriter, req *http.Request) bool {
	serverAuth := backend.GetInfo().ServerAuth

	if serverAuth == nil {
		return false
	}

	cookie, err := req.Cookie(SERVER_AUTH_COOKIE)
	if err == nil {
		payload, err := NewTimestampSigner(sha1.New()).Verify(cookie.Value, SERVER_AUTH_DURATION)
		if err == nil {
			if payload == getCookieToken(backend.GetInfo()) {
				return false
			}
		}
	}

	redirect_uri := getScheme(req) + "://" + req.Host + req.URL.Path + SERVER_AUTH_ENDPOINT
	redirect := serverAuth.AuthUrl + "?redirect_uri=" + url.QueryEscape(redirect_uri)
	http.Redirect(w, req, redirect, 302)
	return true
}
