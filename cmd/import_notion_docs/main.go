package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/russross/blackfriday"
)

const (
	// this name changes for every export operation
	srcZip = "/Users/kjk/Downloads/Export-4da5d4d6a9d747dc9438b5f6d6b3de90.zip"
)

func main() {
	htmlDir := filepath.Join("www", "docs")
	mdDir := filepath.Join(htmlDir, "md")
	err := os.MkdirAll(mdDir, 0755)
	fatalIfErr(err)
	f, err := os.Open(srcZip)
	fatalIfErr(err)
	defer f.Close()
	size := fileSizeMust(srcZip)
	zr, err := zip.NewReader(f, size)
	fatalIfErr(err)
	for _, f := range zr.File {
		d := readZipFileMust(f)
		fmt.Printf("Read %s of size %d\n", f.Name, len(d))
		mdPath := filepath.Join(mdDir, f.Name)

		// replace .md links with .html links. Should be more sophisticated than this
		d = bytes.Replace(d, []byte(".md"), []byte(".html"), -1)
		// change the first h1 in the form of
		// # Command-line arguments
		// to one that also contains link to main page
		// # [Docs](/docs/) : ommand-line arguments
		// except for the index page
		if f.Name != "SumatraPDF-documentation-fed36a5624d443fe9f7be0e410ecd715.md" {
			d = bytes.Replace(d, []byte("# "), []byte("# [Documentation](/docs/) : "), 1)
		}
		// some links in exported .md files are absolute to www.notion.so.
		// change them to be relative to docs folder
		// TODO: they don't have .md suffix, so we should add it here
		// for now we fix it up in serving code
		d = bytes.Replace(d, []byte("https://www.notion.so"), []byte("/docs"), -1)
		d = bytes.Replace(d, []byte("http://www.sumatrapdfreader.org"), []byte("//www.sumatrapdfreader.org"), -1)

		err = ioutil.WriteFile(mdPath, d, 0644)
		fatalIfErr(err)

		htmlInner := blackfriday.MarkdownCommon(d)
		html := strings.Replace(string(htmlTmpl), "{{ body }}", string(htmlInner), -1)
		htmlPath := filepath.Join(htmlDir, strings.Replace(f.Name, ".md", ".html", -1))
		err = ioutil.WriteFile(htmlPath, []byte(html), 0644)
		fatalIfErr(err)
	}
}

const (
	htmlTmpl = `<!DOCTYPE html>
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

func readZipFileMust(f *zip.File) []byte {
	rc, err := f.Open()
	fatalIfErr(err)
	defer rc.Close()
	d, err := ioutil.ReadAll(rc)
	fatalIfErr(err)
	return d
}

func fileSizeMust(path string) int64 {
	stat, err := os.Stat(path)
	fatalIfErr(err)
	return stat.Size()
}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatalf("Error: %s\n", err)
	}
}
