package main

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kjk/u"
)

func servePlainText(w http.ResponseWriter, s string) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Length", strconv.Itoa(len(s)))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(s))
}

// returns true if did redirect url
func redirectIfNeeded(w http.ResponseWriter, r *http.Request) bool {
	parsed, err := url.Parse(r.URL.Path)
	if err != nil {
		return false
	}

	redirect := ""
	switch parsed.Path {
	case "/docs/":
		redirect = "/docs/SumatraPDF-documentation-fed36a5624d443fe9f7be0e410ecd715.html"
	case "/":
		redirect = "free-pdf-reader.html"
	case "/download.html":
		redirect = "download-free-pdf-viewer.html"
	case "/forum.html":
		redirect = "https://forum.sumatrapdfreader.org/"
	}

	if redirect == "" {
		return false
	}

	http.Redirect(w, r, redirect, http.StatusFound)
	return true
}

// we used to have translated pages like /free-pdf-reader-bg.html
// but I had to remove them. We want to redirect them to /free-pdf-reader.html
// if that is a valid file.
// This function returns a url to redirect to or "" if can't find valid redirect
func untranslatedURLRedirect(uri string) string {
	// remove '.html' from the end
	newURI := strings.TrimSuffix(uri, ".html")
	if newURI == uri {
		return ""
	}
	// check it ends with '-xx'
	n := len(newURI)
	if n < 4 || uri[n-3] != '-' {
		return ""
	}
	newURI = newURI[:n-3] + ".html"
	path := filepath.Join("www", newURI)
	if u.FileExists(path) {
		fmt.Printf("%s => %s\n", uri, newURI)
		return newURI
	}
	return ""
}

func handleDl(w http.ResponseWriter, r *http.Request) {
	uri := r.URL.Path
	name := uri[len("/dl/"):]
	path := filepath.Join("www", "files", name)
	if !disableLocalDownloads && u.FileExists(path) {
		//fmt.Printf("serving name: '%s' uri: '%s' from local file '%s'\n", name, uri, path)
		http.ServeFile(w, r, path)
		return
	}
	redirectURI := s3Prefix + name
	//fmt.Printf("serving  name: '%s' uri: '%s' by redirecting to '%s' because %s doesn't exist\n", name, uri, redirectURI, path)
	http.Redirect(w, r, redirectURI, http.StatusFound)
}

func serveIfFileExists(w http.ResponseWriter, r *http.Request, path string) bool {
	if !u.FileExists(path) {
		return false
	}
	http.ServeFile(w, r, path)
	return true
}

// /ping
func handlePing(w http.ResponseWriter, r *http.Request) {
	servePlainText(w, "pong")
}

// /
func handleMainPage(w http.ResponseWriter, r *http.Request) {
	if redirectIfNeeded(w, r) {
		return
	}

	parsed, err := url.Parse(r.URL.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	uri := parsed.Path
	path := filepath.Join("www", uri)

	if serveIfFileExists(w, r, path) {
		return
	}

	// some links in /docs/ don't have .html suffix, so re-try by adding it
	if serveIfFileExists(w, r, path+".html") {
		return
	}

	newURI := untranslatedURLRedirect(uri)
	if newURI != "" {
		http.Redirect(w, r, newURI, http.StatusFound)
		return
	}

	// TODO: custom 404 page
	http.NotFound(w, r)
}

func makeHTTPServer() *http.Server {
	mux := &http.ServeMux{}

	mux.HandleFunc("/", handleMainPage)
	mux.HandleFunc("/ping", handlePing)
	mux.HandleFunc("/dl/", handleDl)

	// https://blog.gopheracademy.com/advent-2016/exposing-go-on-the-internet/
	srv := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      mux,
	}
	return srv
}
