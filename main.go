package main

import (
	"fmt"
	"net/http"
)

func main() {
	sfu := NewSFU()

	// Websocket handle func
	http.HandleFunc("/ws", sfu.ws)

	// web handle func
	http.Handle("/", http.FileServer(http.Dir("./static")))

	// Support https, so we can test by lan
	fmt.Printf("Listening on https://localhost:%d\n", Port())
	panic(http.ListenAndServeTLS(fmt.Sprintf(":%d", Port()), TLSCert(), TLSKey(), nil))
}
