package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/kjk/u"
	"github.com/russross/blackfriday"
)

const (
	htmlTmpl = `<!doctype html>
<html>

<head>
	<meta http-equiv="Content-Language" content="en-us">
	<meta http-equiv="Content-Type" content="text/html; charset=utf-8">
	<meta name="keywords" content="pdf, epub, mobi, chm, cbr, cbz, xps, djvu, reader, viewer" />
	<meta name="description" content="Sumatra PDF reader and viewer for Windows" />
	<title>Sumatra PDF Documentation</title>
	<link rel="stylesheet" href="/sumatra.css" type="text/css" />
</head>

<body>
	<script type="text/javascript" src="/sumatra.js"></script>

	<div id="container">
		<div id="banner">
			<h1 style="display:inline;">Sumatra PDF
				<font size="-1">is a PDF, ePub, MOBI, CHM, XPS, DjVu, CBZ, CBR reader for Windows</font>
			</h1>
			<script type="text/javascript">
				document.write(navHtml());
			</script>
		</div>

		<br/>

		<div id="center">
			<div class="content">
				{{ body }}
			</div>
		</div>
	</div>

	<hr>
	<center><a href="https://blog.kowalczyk.info">Krzysztof Kowalczyk</a></center>
	<script>
		window.ga = window.ga || function() {
			(ga.q = ga.q || []).push(arguments)
		};
		ga.l = +new Date;
		ga('create', 'UA-194516-5', 'auto');
		ga('send', 'pageview');
	</script>
	<script async src="//www.google-analytics.com/analytics.js"></script>
</body>
</html>
`
)

func isMdFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md"
}

func isHTMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".html"
}

func getDocsMdFiles() []string {
	var res []string
	dir := filepath.Join("www", "docs", "md")
	fileInfos, err := ioutil.ReadDir(dir)
	u.PanicIfErr(err)
	for _, fi := range fileInfos {
		if fi.IsDir() || !fi.Mode().IsRegular() {
			continue
		}
		if !isMdFile(fi.Name()) {
			continue
		}

		path := filepath.Join(dir, fi.Name())
		res = append(res, path)
	}
	return res
}

func removeDocsHTML() {
	dir := filepath.Join("www", "docs")
	fileInfos, err := ioutil.ReadDir(dir)
	u.PanicIfErr(err)
	for _, fi := range fileInfos {
		if fi.IsDir() || !fi.Mode().IsRegular() {
			continue
		}
		if !isHTMLFile(fi.Name()) {
			continue
		}
		path := filepath.Join(dir, fi.Name())
		err = os.Remove(path)
		u.PanicIfErr(err)
	}
}

func docsHTMLPath(mdPath string) string {
	dir := filepath.Join("www", "docs")
	name := filepath.Base(mdPath)
	name = replaceExt(name, ".html")
	return filepath.Join(dir, name)
}

func genDocs() {
	removeDocsHTML()
	files := getDocsMdFiles()
	for _, mdFile := range files {
		d, err := ioutil.ReadFile(mdFile)
		u.PanicIfErr(err)

		htmlInner := blackfriday.MarkdownCommon(d)
		html := strings.Replace(string(htmlTmpl), "{{ body }}", string(htmlInner), -1)
		htmlPath := docsHTMLPath(mdFile)
		err = ioutil.WriteFile(htmlPath, []byte(html), 0644)
		u.PanicIfErr(err)
		fmt.Printf("%s => %s\n", mdFile, htmlPath)
	}
}
