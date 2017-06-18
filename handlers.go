package main

import (
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

var (
	html404 = `
<!doctype html>
<html>
<head>
  <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
  <title>SumatraPDF Reader - page not found</title>
	<link rel="stylesheet" href="sumatra.css" type="text/css">
</head>
<body>
  <div style="margin-top:64px; margin-left:auto; margin-right:auto; max-width:800px">
    <p style="color:red">Page <tt>${url}</tt> doesn't exist!</p>
    <p>Try:
      <ul>
        <li><a href="/">Home</a></li>
      </ul>
    </p>
  </div>
</body>
</html>
`
)

func serve404(w http.ResponseWriter, r *http.Request) {
	uri := r.URL.Path
	s := strings.Replace(html404, "${url}", uri, -1)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(s)))
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(s))
}

func shouldLog404(uri string) bool {
	ext := strings.ToLower(filepath.Ext(uri))
	switch ext {
	case ".php", ".gz", ".bak", ".gz", ".ssi", ".tar", ".rar", ".sql":
		return false
	case ".zip":
		return strings.HasPrefix("/dl/", uri)
	}
	return true
}

var (
	redirects = map[string]string{
		"/docs/":                             "/docs/SumatraPDF-documentation-fed36a5624d443fe9f7be0e410ecd715.html",
		"/":                                  "free-pdf-reader.html",
		"/index.html":                        "free-pdf-reader.html",
		"/index.php":                         "free-pdf-reader.html",
		"/index.htm":                         "free-pdf-reader.html",
		"/home.php":                          "free-pdf-reader.html",
		"/free-pdf-reader.html:":             "free-pdf-reader.html",
		"/free-pdf-reader-ja.htmlPDF":        "free-pdf-reader.html",
		"/free-pdf-reader-ru.html/":          "free-pdf-reader.html",
		"/sumatrapdf":                        "free-pdf-reader.html",
		"/download.html":                     "download-free-pdf-viewer.html",
		"/download-free-pdf-viewer-es.html,": "download-free-pdf-viewer.html",
		"/translators.html":                  "https://github.com/sumatrapdfreader/sumatrapdf/blob/master/TRANSLATORS",
		"/develop.html/":                     "/docs/Join-the-project-as-a-developer-be6ef085e89f49038c2b671c0743b690.html",
		"/develop.html":                      "/docs/Join-the-project-as-a-developer-be6ef085e89f49038c2b671c0743b690.html",
		"/forum.html":                        "https://forum.sumatrapdfreader.org/",
	}
)

// returns true if did redirect url
func redirectIfNeeded(w http.ResponseWriter, r *http.Request) bool {
	parsed, err := url.Parse(r.URL.Path)
	if err != nil {
		return false
	}

	redirect := redirects[parsed.Path]
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
		// fmt.Printf("%s => %s\n", uri, newURI)
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

	if shouldLog404(uri) {
		serve404(w, r)
	}
}

func makeHTTPServer() *http.Server {
	mux := &http.ServeMux{}

	mux.HandleFunc("/", withAnalyticsLogging(handleMainPage))
	mux.HandleFunc("/ping", handlePing)
	mux.HandleFunc("/dl/", withAnalyticsLogging(handleDl))

	// https://blog.gopheracademy.com/advent-2016/exposing-go-on-the-internet/
	srv := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      mux,
	}
	return srv
}
