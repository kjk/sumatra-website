package main

import (
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

	"github.com/kjk/sumatra-website/pkg/loggly"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
)

const (
	s3Prefix = "https://kjkpub.s3.amazonaws.com/sumatrapdf/rel/"
)

var (
	httpAddr     string
	inProduction bool
	logglyToken  string
	lggly        *loggly.Client
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
	if fileExists(path) {
		//fmt.Printf("serving '%s' from local file '%s'\n", uri, path)
		http.ServeFile(w, r, path)
		return
	}
	redirectURI := s3Prefix + name
	//fmt.Printf("serving '%s' by redirecting to '%s'\n", uri, redirectURI)
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

func initHTTPHandlers() {
	http.HandleFunc("/", handleMainPage)
	http.HandleFunc("/dl/", handleDl)
}

func findMyProcess() *process.Process {
	pids, err := process.Pids()
	if err != nil {
		return nil
	}
	for _, pid := range pids {
		proc, err := process.NewProcess(pid)
		if err != nil {
			continue
		}
		name, err := proc.Name()
		if err != nil {
			continue
		}
		name = strings.ToLower(name)
		switch name {
		case "sumatra_website_linux", "sumatra_website":
			return proc
		}
	}
	return nil
}

func logMemUsage() {
	var args []interface{}
	mem, err := mem.VirtualMemory()
	if err == nil {
		args = append(args, "mem-cached", mem.Cached, "mem-buffers", mem.Buffers, "mem-used", mem.Used, "mem-free", mem.Free)
	}
	proc := findMyProcess()
	if proc != nil {
		memInfo, err := proc.MemoryInfo()
		if err == nil {
			args = append(args, "proc-mem-rss", memInfo.RSS)
		}
	}
	//fmt.Printf("%v\n", args)
	err = lggly.Log(args...)
	if err != nil {
		fmt.Printf("lggly.Log failed with %s\n", err)
	}
}

func logMemUsageWorker() {
	for {
		logMemUsage()
		time.Sleep(time.Minute)
	}
}
func main() {
	logglyToken = strings.TrimSpace(os.Getenv("LOGGLY_TOKEN"))
	parseCmdLineFlags()
	rand.Seed(time.Now().UnixNano())

	if logglyToken != "" {
		fmt.Printf("Got loggly token so will send data to loggly\n")
		lggly = loggly.New(logglyToken, "sumatra-website")
		if inProduction {
			lggly.Tag("production")
		} else {
			lggly.Tag("dev")
		}
		go logMemUsageWorker()
	}

	initHTTPHandlers()
	msg := fmt.Sprintf("Started running on %s, inProduction: %v", httpAddr, inProduction)
	fmt.Printf("%s\n", msg)
	lggly.Log("log", msg)
	if err := http.ListenAndServe(httpAddr, nil); err != nil {
		fmt.Printf("http.ListendAndServer() failed with %s\n", err)
	}
	fmt.Printf("Exited\n")
}
