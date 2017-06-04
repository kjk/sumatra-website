package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/SparkPost/gosparkpost"
	"github.com/kjk/u"
)

const (
	mailFrom = "Sumatra Website Stats <info@kjktools.org>"
	mailTo   = "kkowalczyk@gmail.com"
)

func sendMail(subject, body string) error {
	sparkpostKey := strings.TrimSpace(os.Getenv("SPARK_POST_KEY"))
	if sparkpostKey == "" {
		return nil
	}

	cfg := &gosparkpost.Config{
		BaseUrl:    "https://api.sparkpost.com",
		ApiKey:     sparkpostKey,
		ApiVersion: 1,
	}

	var sparky gosparkpost.Client
	err := sparky.Init(cfg)
	if err != nil {
		return err
	}
	sparky.Client = http.DefaultClient

	tx := &gosparkpost.Transmission{
		Recipients: []string{mailTo},
		Content: gosparkpost.Content{
			Text:    body,
			From:    mailFrom,
			Subject: subject,
		},
	}
	_, _, err = sparky.Send(tx)
	return err
}

func sendBootMail() {
	subject := u.UtcNow().Format("sumatra website started on 2006-01-02 15:04:05")
	body := "Just letting you know that I've started\n"
	body += fmt.Sprintf("production: %v, data dir: %s, ver: github.com/sumatrapdfreader/sumatra-website/commit/%s\n", flgProduction, getDataDir(), sha1ver)
	sendMail(subject, body)
}
