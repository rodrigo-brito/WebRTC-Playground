package main

import (
	"embed"
	"fmt"
	"net/http"
)

// go:embed static/*
var static embed.FS

func main() {
	sfu := NewSFU()

	// Websocket handle func
	http.HandleFunc("/ws", sfu.ws)

	// web handle func
	http.Handle("/*", http.FileServer(http.FS(static)))

	// Support https, so we can test by lan
	fmt.Printf("Listening on https://localhost:%d\n", Port())
	panic(http.ListenAndServeTLS(fmt.Sprintf(":%d", Port()), TLSCert(), TLSKey(), nil))
}
