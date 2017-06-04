package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
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
	httpAddr     string
	inProduction bool
	// if true, we redirect all downloads to s3. If false, some of them
	// will be served by us (and cached by cloudflare)
	disableLocalDownloads = false
)

func parseCmdLineFlags() {
	flag.StringVar(&httpAddr, "addr", "127.0.0.1:5030", "HTTP server address")
	flag.BoolVar(&inProduction, "production", false, "are we running in production")
	flag.Parse()
	if inProduction {
		httpAddr = ":80"
	}
}

func logIfErr(err error) {
	if err != nil {
		fmt.Printf("error: '%s'\n", err)
	}
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
			u.PanicIfErr(err)
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
		u.PanicIfErr(err)
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
