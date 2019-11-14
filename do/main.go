package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/kjk/u"
)

func runLocal() {
	exeName := "sumatra_website.exe"
	// build the website generator
	{
		cmd := exec.Command("go", "build", "-o", exeName)
		u.RunCmdLoggedMust(cmd)
	}
	// generate files
	{
		cmd := exec.Command("./" + exeName)
		u.RunCmdLoggedMust(cmd)
	}
	os.Remove(exeName)
	// run caddy to preview the website, on localhost:9000
	{
		cmd := exec.Command("caddy", "-log", "stdout")
		cmd.Stdout = os.Stdout
		cmd.Start()
		u.OpenBrowser("http://localhost:9000")
		cmd.Wait()
	}
}

func deploy() {
	fmt.Printf("deploy\n")
}

func logf(format string, args ...interface{}) {
	s := format
	if len(args) > 0 {
		s = fmt.Sprintf(format, args...)
	}
	fmt.Print(s)
}

func main() {
	u.CdUpDir("sumatra-website")
	logf("dir: '%s'\n", u.CurrDirAbsMust())

	var (
		flgRun    bool
		flgDeploy bool
	)
	flag.BoolVar(&flgRun, "run", false, "run webserver locally to preview the changes")
	flag.BoolVar(&flgDeploy, "deploy", false, "deploy to Netlify")
	flag.Parse()

	if flgRun {
		runLocal()
		return
	}
	if flgDeploy {
		deploy()
		return
	}

	flag.Usage()
}
