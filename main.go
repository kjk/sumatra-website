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
	"sync/atomic"
	"time"

	"github.com/kjk/sumatra-website/pkg/loggly"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
)

const (
	s3Prefix = "https://kjkpub.s3.amazonaws.com/sumatrapdf/rel/"
)

var (
	httpAddr               string
	inProduction           bool
	logglyToken            string
	lggly                  *loggly.Client
	nConcurrentConnections int32
	nTotalConnections      int64
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
	atomic.AddInt32(&nConcurrentConnections, 1)
	defer atomic.AddInt32(&nConcurrentConnections, -1)

	atomic.AddInt64(&nTotalConnections, 1)

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
	atomic.AddInt32(&nConcurrentConnections, 1)
	defer atomic.AddInt32(&nConcurrentConnections, -1)

	if redirectIfNeeded(w, r) {
		return
	}

	parsed, err := url.Parse(r.URL.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	atomic.AddInt64(&nTotalConnections, 1)

	fileName := parsed.Path
	path := filepath.Join("www", fileName)
	http.ServeFile(w, r, path)
}

var bitlyLinks = []string{
	"https://smallpdf.com/compress-pdf",
	"http://bit.ly/2hfrJOm",

	"https://smallpdf.com/ppt-to-pdf",
	"http://bit.ly/2hgp3Mh",

	"https://smallpdf.com/pdf-to-ppt",
	"http://bit.ly/2gR7s0k",

	"https://smallpdf.com/jpg-to-pdf",
	"http://bit.ly/2h1f127",

	"https://smallpdf.com/pdf-to-jpg",
	"http://bit.ly/2gAZGoo",

	"https://smallpdf.com/excel-to-pdf",
	"http://bit.ly/2gR8CJs",

	"https://smallpdf.com/pdf-to-excel",
	"http://bit.ly/2h43L5K",

	"https://smallpdf.com/word-to-pdf",
	"http://bit.ly/2gR3kxF",

	"https://smallpdf.com/pdf-to-word",
	"http://bit.ly/2g8QzKN",

	"https://smallpdf.com/merge-pdf",
	"http://bit.ly/2g8YrvJ",

	"https://smallpdf.com/split-pdf",
	"http://bit.ly/2hgrfDq",

	"https://smallpdf.com/rotate-pdf",
	"http://bit.ly/2g93tbJ",

	"https://smallpdf.com/unlock-pdf",
	"http://bit.ly/2gAWkBQ",

	"https://smallpdf.com/protect-pdf",
	"http://bit.ly/2g8eTBu",
}

func findSmallPdfDst(s string) string {
	n := len(bitlyLinks) / 2
	for i := 0; i < n; i++ {
		l := bitlyLinks[i*2]
		if strings.HasSuffix(l, s) {
			return bitlyLinks[i*2+1]
		}
	}
	return ""
}

func handleGoTo(w http.ResponseWriter, r *http.Request) {
	dst := r.URL.Path[len("/go-to/"):]
	fmt.Printf("url: %s, goto: %s\n", r.URL.Path, dst)
	redirect := findSmallPdfDst(dst)
	if redirect == "" {
		// shouldn't happen
		redirect = "/pdf-tools.html"
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

func initHTTPHandlers() {
	http.HandleFunc("/", handleMainPage)
	http.HandleFunc("/dl/", handleDl)
	http.HandleFunc("/go-to/", handleGoTo)
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
	nConn := atomic.LoadInt32(&nConcurrentConnections)
	nTotalConn := atomic.LoadInt64(&nTotalConnections)

	args := []interface{}{"ntotalconnections", nTotalConn, "nconcurrentconnections", nConn}

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
	if lggly != nil {
		err = lggly.Log(args...)
		if err != nil {
			fmt.Printf("lggly.Log failed with %s\n", err)
		}
	}

	fmt.Printf("%v\n", args)
}

func logMemUsageWorker() {
	for {
		logMemUsage()
		time.Sleep(time.Minute * 10)
	}
}

func main() {
	logglyToken = strings.TrimSpace(os.Getenv("LOGGLY_TOKEN"))
	parseCmdLineFlags()
	rand.Seed(time.Now().UnixNano())

	if logglyToken != "" {
		fmt.Printf("Got loggly token '%s' so will send data to loggly\n", logglyToken)
		lggly = loggly.New(logglyToken, "sumatra-website")
		if inProduction {
			lggly.Tag("production")
		} else {
			lggly.Tag("dev")
		}
	}
	go logMemUsageWorker()

	initHTTPHandlers()
	msg := fmt.Sprintf("Started running on %s, inProduction: %v", httpAddr, inProduction)
	fmt.Printf("%s\n", msg)
	lggly.Log("log", msg)
	if err := http.ListenAndServe(httpAddr, nil); err != nil {
		fmt.Printf("http.ListendAndServer() failed with %s\n", err)
	}
	fmt.Printf("Exited\n")
}
