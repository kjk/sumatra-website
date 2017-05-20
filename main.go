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
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
	if inProduction {
		httpAddr = ":80"
	}
}

// return true if redirected
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

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
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
	if fileExists(path) {
		fmt.Printf("%s => %s\n", uri, newURI)
		return newURI
	}
	return ""
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

func serveIfFileExists(w http.ResponseWriter, r *http.Request, path string) bool {
	if !fileExists(path) {
		return false
	}
	http.ServeFile(w, r, path)
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

func logIfErr(err error) {
	if err != nil {
		fmt.Printf("error: '%s'\n", err)
	}
}

func fmtArgs(args ...interface{}) string {
	if len(args) == 0 {
		return ""
	}
	format := args[0].(string)
	if len(args) == 1 {
		return format
	}
	return fmt.Sprintf(format, args[1:]...)
}

func panicWithMsg(defaultMsg string, args ...interface{}) {
	s := fmtArgs(args...)
	if s == "" {
		s = defaultMsg
	}
	fmt.Printf("%s\n", s)
	panic(s)
}

func fatalIfErr(err error, args ...interface{}) {
	if err == nil {
		return
	}
	panicWithMsg(err.Error(), args...)
}

func fatalIf(cond bool, args ...interface{}) {
	if !cond {
		return
	}
	panicWithMsg("fatalIf: condition failed", args...)
}

func makeHTTPServer() *http.Server {
	mux := &http.ServeMux{}

	mux.HandleFunc("/", handleMainPage)
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

func hostPolicy(ctx context.Context, host string) error {
	if strings.HasSuffix(host, "sumatrapdfreader.org") {
		return nil
	}
	return errors.New("acme/autocert: only *.sumatrapdfreader.org hosts are allowed")
}

func main() {
	parseCmdLineFlags()
	rand.Seed(time.Now().UnixNano())

	var wg sync.WaitGroup
	var httpsSrv, httpSrv *http.Server

	if inProduction {
		httpsSrv = makeHTTPServer()
		m := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: hostPolicy,
		}
		httpsSrv.Addr = ":443"
		httpsSrv.TLSConfig = &tls.Config{GetCertificate: m.GetCertificate}
		fmt.Printf("Started runing HTTPS on %s\n", httpsSrv.Addr)
		go func() {
			wg.Add(1)
			err := httpsSrv.ListenAndServeTLS("", "")
			// mute error caused by Shutdown()
			if err == http.ErrServerClosed {
				err = nil
			}
			fatalIfErr(err)
			fmt.Printf("HTTPS server gracefully stopped\n")
			wg.Done()
		}()
	}

	httpSrv = makeHTTPServer()
	httpSrv.Addr = httpAddr
	fmt.Printf("Started running on %s, inProduction: %v\n", httpAddr, inProduction)
	go func() {
		wg.Add(1)
		err := httpSrv.ListenAndServe()
		// mute error caused by Shutdown()
		if err == http.ErrServerClosed {
			err = nil
		}
		fatalIfErr(err)
		fmt.Printf("HTTP server gracefully stopped\n")
		wg.Done()
	}()

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt /* SIGINT */, syscall.SIGTERM)
	sig := <-c
	fmt.Printf("Got signal %s\n", sig)
	if httpsSrv != nil {
		httpsSrv.Shutdown(nil)
	}
	if httpSrv != nil {
		httpSrv.Shutdown(nil)
	}
	wg.Wait()
	fmt.Printf("Did shutdown http and https servers\n")
}
