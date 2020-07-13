package main

import (
	"fmt"
	"net/http"

	"github.com/pion/webrtc/v2"
)

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	// Create a MediaEngine object to configure the supported codec
	m = webrtc.MediaEngine{}

	// Setup the codecs you want to use.
	m.RegisterCodec(webrtc.NewRTPVP8Codec(webrtc.DefaultPayloadTypeVP8, 90000))
	m.RegisterCodec(webrtc.NewRTPOpusCodec(webrtc.DefaultPayloadTypeOpus, 48000))

	// Create the API object with the MediaEngine
	api = webrtc.NewAPI(webrtc.WithMediaEngine(m))

	// Websocket handle func
	http.HandleFunc("/ws", ws)

	// web handle func
	http.Handle("/", http.FileServer(http.Dir("./static")))

	// Support https, so we can test by lan
	fmt.Printf("Listening on https://localhost:%d\n", Port())
	panic(http.ListenAndServeTLS(fmt.Sprintf(":%d", Port()), TLSCert(), TLSKey(), nil))
}
