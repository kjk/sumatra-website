package main

import (
	"math/rand"
	"time"
)

const (
	s3Prefix = "https://kjkpub.s3.amazonaws.com/sumatrapdf/rel/"
)

func downloadSumatraFiles() {
	// TODO: write me
}

func main() {
	rand.Seed(time.Now().UnixNano())

	downloadSumatraFiles()

	genDocs()
	netlifyBuild()
}
