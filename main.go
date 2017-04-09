package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/acme/autocert"

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
	nTotalDownloads        int64
	disableLocalDownloads  = false
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
	if !disableLocalDownloads && fileExists(path) {
		atomic.AddInt64(&nTotalDownloads, 1)
		//fmt.Printf("serving name: '%s' uri: '%s' from local file '%s'\n", name, uri, path)
		http.ServeFile(w, r, path)
		return
	}
	redirectURI := s3Prefix + name
	//fmt.Printf("serving  name: '%s' uri: '%s' by redirecting to '%s' because %s doesn't exist\n", name, uri, redirectURI, path)
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

func userHomeDir() string {
	// user.Current() returns nil if cross-compiled e.g. on mac for linux
	if usr, _ := user.Current(); usr != nil {
		return usr.HomeDir
	}
	return os.Getenv("HOME")
}

func expandTildeInPath(s string) string {
	if strings.HasPrefix(s, "~") {
		return userHomeDir() + s[1:]
	}
	return s
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func getDataDir() string {
	for _, dir := range []string{"~/data/sumatra-website", "/data"} {
		dir = expandTildeInPath(dir)
		if pathExists(dir) {
			return dir
		}
	}
	return ""
}

func statsFilePath() string {
	return filepath.Join(getDataDir(), "stats.json")
}

// types:
// tc - tool click

type statToolClick struct {
	Type      string    `json:"t"`
	Timestamp time.Time `json:"ts"`
	Where     string    `json:"w"`
}

var (
	statsFile   *os.File
	muStatsFile sync.Mutex
	stats       map[string]int
)

func init() {
	stats = make(map[string]int)
}

func logIfErr(err error) {
	if err != nil {
		fmt.Printf("error: '%s'\n", err)
	}
}

func openStatsFile() {
	var err error
	statsFile, err = os.OpenFile(statsFilePath(), os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	logIfErr(err)
}

func readStats() {
	f, err := os.Open(statsFilePath())
	if err != nil {
		logIfErr(err)
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := scanner.Text()
		s = strings.TrimSpace(s)
		var st statToolClick
		err = json.Unmarshal([]byte(s), &st)
		if err != nil {
			logIfErr(err)
			continue
		}
		stats[st.Where]++
	}
}

func recordToolClick(where string) {
	muStatsFile.Lock()
	defer muStatsFile.Unlock()

	stats[where]++
	if statsFile == nil {
		return
	}
	e := &statToolClick{
		Type:      "tc",
		Timestamp: time.Now(),
		Where:     where,
	}
	d, err := json.Marshal(e)
	if err != nil {
		logIfErr(err)
		return
	}
	_, err = statsFile.Write(d)
	logIfErr(err)
	_, err = statsFile.WriteString("\n")
	logIfErr(err)
	err = statsFile.Sync()
	logIfErr(err)
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
	redirect := findSmallPdfDst(dst)
	if redirect == "" {
		// shouldn't happen
		redirect = "/pdf-tools.html"
	} else {
		recordToolClick(dst)
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

type whereCount struct {
	Where string
	Count int
}

type whereCountByCount []whereCount

func (a whereCountByCount) Len() int { return len(a) }
func (a whereCountByCount) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a whereCountByCount) Less(i, j int) bool {
	return a[j].Count < a[i].Count
}

func getSortedStats() []whereCount {
	muStatsFile.Lock()
	defer muStatsFile.Unlock()
	var res []whereCount
	for where, count := range stats {
		res = append(res, whereCount{where, count})
	}
	sort.Sort(whereCountByCount(res))
	return res
}

var (
	statsTmpl = `<!doctype html><html><body>
		<div>Stats of what people click</div>
		<table>
			<thead>
				<tr>
					<td>Where</td>
					<td>Count</td>
				</tr>
			</thead>

			<tbody>
				{{ range .Stats }}
				<tr>
						<td>{{ .Where }}</td>
						<td>{{ .Count }}</td>
				</tr>
				{{ end }}
			<tbody>
		</table>
	</body></html>`
)

func execTemplateString(w http.ResponseWriter, templateString string, v interface{}) {
	tmpl := template.New("stats")
	tmpl, err := tmpl.Parse(templateString)
	if err != nil {
		logIfErr(err)
		return
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "stats", v); err != nil {
		logIfErr(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// at this point we ignore error
	w.Header().Set("Content-Length", strconv.Itoa(len(buf.Bytes())))
	w.Write(buf.Bytes())
}

func handleSeeStats(w http.ResponseWriter, r *http.Request) {
	//fmt.Printf("handlSeeStats\n")
	stats := getSortedStats()
	v := struct {
		Stats []whereCount
	}{
		Stats: stats,
	}
	execTemplateString(w, statsTmpl, v)
}

// https://blog.gopheracademy.com/advent-2016/exposing-go-on-the-internet/
func makeHTTPServer() *http.Server {
	mux := &http.ServeMux{}

	mux.HandleFunc("/", handleMainPage)
	mux.HandleFunc("/dl/", handleDl)
	mux.HandleFunc("/go-to/", handleGoTo)
	mux.HandleFunc("/see-stats", handleSeeStats)

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
	nTotalDls := atomic.LoadInt64(&nTotalDownloads)

	args := []interface{}{
		"ntotalconnections", nTotalConn,
		"ntotaldls", nTotalDls,
		"nconcurrentconnections", nConn,
	}

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

func hostPolicy(ctx context.Context, host string) error {
	if strings.HasSuffix(host, "sumatrapdfreader.org") {
		return nil
	}
	return errors.New("acme/autocert: only *.sumatrapdfreader.org hosts are allowed")
}

func main() {
	logglyToken = strings.TrimSpace(os.Getenv("LOGGLY_TOKEN"))
	parseCmdLineFlags()
	rand.Seed(time.Now().UnixNano())

	readStats()
	openStatsFile()

	if logglyToken != "" {
		fmt.Printf("Got loggly token '%s' so will send data to loggly\n", logglyToken)
		lggly = loggly.New(logglyToken, "sumatra-website")
		if inProduction {
			lggly.Tag("production")
		} else {
			lggly.Tag("dev")
		}
	}
	// go logMemUsageWorker()

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
	lggly.Log("log", msg)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("http.ListendAndServer() failed with %s\n", err)
	}
	fmt.Printf("Exited\n")
}
