package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	log "github.com/sirupsen/logrus"
)

//go:embed static/*
var static embed.FS

func main() {
	staticDir, err := fs.Sub(static, "static")
	if err != nil {
		log.Fatal(err)
	}

	sfu := NewSFU()

	// Websocket handle func
	http.HandleFunc("/ws", sfu.ws)

	// web handle func
	http.Handle("/", http.FileServer(http.FS(staticDir)))

	// Support https, so we can test by lan
	fmt.Printf("Listening on https://localhost:%d\n", Port())
	panic(http.ListenAndServeTLS(fmt.Sprintf(":%d", Port()), TLSCert(), TLSKey(), nil))
}
