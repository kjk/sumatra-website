package main

import (
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

var (
	analyticsCode = "UA-194516-5"
)

func writeResponse(w http.ResponseWriter, responseBody string) {
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(responseBody)), 10))
	io.WriteString(w, responseBody)
}

func textResponse(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/plain")
	writeResponse(w, text)
}

var (
	httpAddr     string
	inProduction bool
)

func parseCmdLineFlags() {
	flag.StringVar(&httpAddr, "addr", "127.0.0.1:5030", "HTTP server address")
	flag.BoolVar(&inProduction, "production", false, "are we running in production")
	flag.Parse()
}

func redirectIfNeeded(w http.ResponseWriter, r *http.Request) bool {
	parsed, err := url.Parse(r.URL.Path)
	if err != nil {
		return false
	}

	redirect := ""
	switch parsed.Path {
	case "/":
		redirect = "free-pdf-reader.html"
	case "/download.html":
		redirect = "download-free-pdf-viewer.html"
	}

	if redirect == "" {
		return false
	}

	http.Redirect(w, r, redirect, 302)
	return true
}

func handleMainPage(w http.ResponseWriter, r *http.Request) {
	if redirectIfNeeded(w, r) {
		return
	}

	parsed, err := url.Parse(r.URL.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	fileName := parsed.Path
	path := filepath.Join("www", fileName)
	http.ServeFile(w, r, path)
}

func initHTTPHandlers() {
	http.HandleFunc("/", handleMainPage)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	parseCmdLineFlags()

	rand.Seed(time.Now().UnixNano())

	if !inProduction {
		analyticsCode = ""
	}

	initHTTPHandlers()
	fmt.Printf("Started runing on %s\n", httpAddr)
	if err := http.ListenAndServe(httpAddr, nil); err != nil {
		fmt.Printf("http.ListendAndServer() failed with %s\n", err)
	}
	fmt.Printf("Exited\n")
}
