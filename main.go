package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

const (
	s3Prefix = "https://kjkpub.s3.amazonaws.com/sumatrapdf/rel/"
)

var (
	httpAddr              string
	inProduction          bool
	disableLocalDownloads = false
)

func writeResponse(w http.ResponseWriter, responseBody string) {
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(responseBody)), 10))
	io.WriteString(w, responseBody)
}

func textResponse(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/plain")
	writeResponse(w, text)
}

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

	http.Redirect(w, r, redirect, http.StatusFound)
	return true
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

func handleDl(w http.ResponseWriter, r *http.Request) {
	uri := r.URL.Path
	name := uri[len("/dl/"):]
	path := filepath.Join("www", "files", name)
	if !disableLocalDownloads && fileExists(path) {
		//fmt.Printf("serving name: '%s' uri: '%s' from local file '%s'\n", name, uri, path)
		http.ServeFile(w, r, path)
		return
	}
	redirectURI := s3Prefix + name
	//fmt.Printf("serving  name: '%s' uri: '%s' by redirecting to '%s' because %s doesn't exist\n", name, uri, redirectURI, path)
	http.Redirect(w, r, redirectURI, http.StatusFound)
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

func logIfErr(err error) {
	if err != nil {
		fmt.Printf("error: '%s'\n", err)
	}
}

// https://blog.gopheracademy.com/advent-2016/exposing-go-on-the-internet/
func makeHTTPServer() *http.Server {
	mux := &http.ServeMux{}

	mux.HandleFunc("/", handleMainPage)
	mux.HandleFunc("/dl/", handleDl)

	srv := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		// TODO: 1.8 only
		// IdleTimeout:  120 * time.Second,
		Handler: mux,
	}
	// TODO: track connections and their state
	return srv
}

func hostPolicy(ctx context.Context, host string) error {
	if strings.HasSuffix(host, "sumatrapdfreader.org") {
		return nil
	}
	return errors.New("acme/autocert: only *.sumatrapdfreader.org hosts are allowed")
}

func main() {
	parseCmdLineFlags()
	rand.Seed(time.Now().UnixNano())

	if inProduction {
		srv := makeHTTPServer()
		m := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: hostPolicy,
		}
		srv.Addr = ":443"
		srv.TLSConfig = &tls.Config{GetCertificate: m.GetCertificate}
		fmt.Printf("Started runing HTTPS on %s\n", srv.Addr)
		go srv.ListenAndServeTLS("", "")
	}

	srv := makeHTTPServer()
	srv.Addr = httpAddr
	msg := fmt.Sprintf("Started running on %s, inProduction: %v", httpAddr, inProduction)
	fmt.Printf("%s\n", msg)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("http.ListendAndServer() failed with %s\n", err)
	}
	fmt.Printf("Exited\n")
}
