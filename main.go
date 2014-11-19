package main

import (
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "github.com/gorilla/mux"
)

var (
	analyticsCode = "UA-194516-1"

	alwaysLogTime = true
)

func StringEmpty(s *string) bool {
	return s == nil || 0 == len(*s)
}

func isTopLevelUrl(url string) bool {
	return 0 == len(url) || "/" == url
}

func getReferer(r *http.Request) string {
	return r.Header.Get("Referer")
}

// Request.RemoteAddress contains port, which we want to remove i.e.:
// "[::1]:58292" => "[::1]"
func ipAddrFromRemoteAddr(s string) string {
	idx := strings.LastIndex(s, ":")
	if idx == -1 {
		return s
	}
	return s[:idx]
}

func getIpAddress(r *http.Request) string {
	hdr := r.Header
	hdrRealIp := hdr.Get("X-Real-Ip")
	hdrForwardedFor := hdr.Get("X-Forwarded-For")
	if hdrRealIp == "" && hdrForwardedFor == "" {
		return ipAddrFromRemoteAddr(r.RemoteAddr)
	}
	if hdrForwardedFor != "" {
		// X-Forwarded-For is potentially a list of addresses separated with ","
		parts := strings.Split(hdrForwardedFor, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}
		// TODO: should return first non-local address
		return parts[0]
	}
	return hdrRealIp
}

func jQueryUrl() string {
	//return "//ajax.googleapis.com/ajax/libs/jquery/1.8.2/jquery.min.js"
	//return "/js/jquery-1.4.2.js"
	return "//cdnjs.cloudflare.com/ajax/libs/jquery/1.8.3/jquery.min.js"
}

func prettifyJsUrl() string {
	//return "//cdnjs.cloudflare.com/ajax/libs/prettify/188.0.0/prettify.js"
	//return "/js/prettify.js"
	return "//cdnjs.cloudflare.com/ajax/libs/prettify/r298/prettify.js"
}

func prettifyCssUrl() string {
	//return "/js/prettify.css"
	return "//cdnjs.cloudflare.com/ajax/libs/prettify/r298/prettify.css"
}

func setContentType(w http.ResponseWriter, contentType string) {
	w.Header().Set("Content-Type", contentType)
}

func writeResponse(w http.ResponseWriter, responseBody string) {
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(responseBody)), 10))
	io.WriteString(w, responseBody)
}

func textResponse(w http.ResponseWriter, text string) {
	setContentType(w, "text/plain")
	writeResponse(w, text)
}

var (
	httpAddr     string
	inProduction bool
)

func parseCmdLineFlags() {
	flag.StringVar(&httpAddr, "addr", ":5020", "HTTP server address")
	flag.BoolVar(&inProduction, "production", false, "are we running in production")
	flag.Parse()
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	var err error

	parseCmdLineFlags()

	if inProduction {
		reloadTemplates = false
		alwaysLogTime = false
	}

	useStdout := !inProduction

	rand.Seed(time.Now().UnixNano())

	if !inProduction {
		analyticsCode = emptyString
	}

	InitHttpHandlers()
	if err := http.ListenAndServe(httpAddr, nil); err != nil {
		fmt.Printf("http.ListendAndServer() failed with %s\n", err)
	}
	fmt.Printf("Exited\n")
}
