package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
)

//go:embed static/*
var static embed.FS

func main() {
	staticDir, err := fs.Sub(static, "static")
	if err != nil {
		log.Fatal(err)
	}

	server := NewServer()
	http.HandleFunc("/ws", server.handle)
	http.Handle("/", http.FileServer(http.FS(staticDir)))

	// Support https, so we can test by lan
	fmt.Printf("Listening on https://localhost:%d\n", Port())
	err = http.ListenAndServeTLS(fmt.Sprintf(":%d", Port()), TLSCert(), TLSKey(), nil)
	if err != nil {
		log.Fatal(err)
	}
}
