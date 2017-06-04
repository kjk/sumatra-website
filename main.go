package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kjk/u"

	"golang.org/x/crypto/acme/autocert"
)

const (
	s3Prefix = "https://kjkpub.s3.amazonaws.com/sumatrapdf/rel/"
)

var (
	flgHTTPAddr   string
	flgProduction bool
	// if true, we redirect all downloads to s3. If false, some of them
	// will be served by us (and cached by cloudflare)
	disableLocalDownloads = false

	dataDir string
	sha1ver string
)

func parseCmdLineFlags() {
	flag.StringVar(&flgHTTPAddr, "addr", "127.0.0.1:5030", "HTTP server address")
	flag.BoolVar(&flgProduction, "production", false, "are we running in production")
	flag.Parse()
	if flgProduction {
		flgHTTPAddr = ":80"
	}
}

func logIfErr(err error) {
	if err != nil {
		fmt.Printf("error: '%s'\n", err)
	}
}

func getDataDir() string {
	if dataDir != "" {
		return dataDir
	}

	dirsToCheck := []string{"/data", u.ExpandTildeInPath("~/data/sumatra-website")}
	for _, dir := range dirsToCheck {
		if u.PathExists(dir) {
			dataDir = dir
			return dataDir
		}
	}

	log.Fatalf("data directory (%v) doesn't exist", dirsToCheck)
	return ""
}

func main() {
	getDataDir() // force early error if data dir doesn't exist

	parseCmdLineFlags()
	rand.Seed(time.Now().UnixNano())

	analyticsPath := filepath.Join(getDataDir(), "analytics", "2006-01-02.txt")
	initAnalyticsMust(analyticsPath)

	var wg sync.WaitGroup
	var httpsSrv, httpSrv *http.Server

	if flgProduction {
		hostPolicy := func(ctx context.Context, host string) error {
			if strings.HasSuffix(host, "sumatrapdfreader.org") {
				return nil
			}
			return errors.New("acme/autocert: only *.sumatrapdfreader.org hosts are allowed")
		}
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
			u.PanicIfErr(err)
			fmt.Printf("HTTPS server gracefully stopped\n")
			wg.Done()
		}()
	}

	httpSrv = makeHTTPServer()
	httpSrv.Addr = flgHTTPAddr
	fmt.Printf("Started running on %s, flgProduction: %v, dataDir: %s\n", flgHTTPAddr, flgProduction, getDataDir())
	go func() {
		wg.Add(1)
		err := httpSrv.ListenAndServe()
		// mute error caused by Shutdown()
		if err == http.ErrServerClosed {
			err = nil
		}
		u.PanicIfErr(err)
		fmt.Printf("HTTP server gracefully stopped\n")
		wg.Done()
	}()

	if flgProduction {
		sendBootMail()
	}

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
