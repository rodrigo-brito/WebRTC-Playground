package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v2/pkg/media/ivfwriter"
	"github.com/pion/webrtc/v2/pkg/media/opuswriter"

	// "github.com/pion/webrtc"
	"github.com/pion/webrtc/v2"
)

// Peer config
var peerConnectionConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	},
	SDPSemantics: webrtc.SDPSemanticsUnifiedPlanWithFallback,
}

var (
	// Media engine
	m webrtc.MediaEngine

	// API object
	api *webrtc.API

	// Publisher Peer
	pcPub *webrtc.PeerConnection

	// Local track
	videoTrackLock = sync.RWMutex{}
	audioTrackLock = sync.RWMutex{}

	// Websocket upgrader
	upgrader = websocket.Upgrader{}

	mediaInfo = make(map[string]avTrack)
)

const (
	rtcpPLIInterval = time.Second * 3
)

type wsMsg struct {
	Type string
	Sdp  string
	Name string
}

type avTrack struct {
	Video *webrtc.Track
	Audio *webrtc.Track
}

func getRcvMedia(name string, media map[string]avTrack) avTrack {
	if v, ok := media[name]; ok {
		return v
	}
	return avTrack{}
}

func ws(w http.ResponseWriter, r *http.Request) {
	// Websocket client
	c, err := upgrader.Upgrade(w, r, nil)
	checkError(err)

	defer func() {
		checkError(c.Close())
	}()

	for {
		// Read sdp from websocket
		mt, msg, err := c.ReadMessage()
		checkError(err)

		wsData := wsMsg{}
		if err := json.Unmarshal(msg, &wsData); err != nil {
			checkError(err)
		}

		sdp := wsData.Sdp
		name := wsData.Name

		m, ok := mediaInfo[name]

		if !ok {
			mediaInfo[name] = avTrack{}
			m = mediaInfo[name]
		}

		if wsData.Type == "publish" {

			// receive chrome publish sdp

			// Create a new RTCPeerConnection
			pcPub, err = api.NewPeerConnection(peerConnectionConfig)
			checkError(err)

			_, err = pcPub.AddTransceiver(webrtc.RTPCodecTypeAudio)
			checkError(err)

			_, err = pcPub.AddTransceiver(webrtc.RTPCodecTypeVideo)
			checkError(err)

			opusFile, err := opuswriter.New(fmt.Sprintf("%s.ogg", name), 48000, 2)
			if err != nil {
				panic(err)
			}
			ivfFile, err := ivfwriter.New(fmt.Sprintf("%s.ivf", name))
			if err != nil {
				panic(err)
			}

			// Set the handler for ICE connection state
			// This will notify you when the peer has connected/disconnected
			pcPub.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
				fmt.Printf("Connection State has changed %s \n", connectionState.String())

				if connectionState == webrtc.ICEConnectionStateConnected {
					fmt.Println("Connected")
				} else if connectionState == webrtc.ICEConnectionStateFailed ||
					connectionState == webrtc.ICEConnectionStateDisconnected {
					err := opusFile.Close()
					if err != nil {
						panic(err)
					}

					err = ivfFile.Close()
					if err != nil {
						panic(err)
					}

					fmt.Println("Disconnected!")
					os.Exit(0)
				}
			})

			// receive av data from chrome
			pcPub.OnTrack(func(remoteTrack *webrtc.Track, receiver *webrtc.RTPReceiver) {
				go func() {
					ticker := time.NewTicker(rtcpPLIInterval)
					for range ticker.C {
						if rtcpSendErr := pcPub.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: remoteTrack.SSRC()}}); rtcpSendErr != nil {
							checkError(rtcpSendErr)
						}
					}
				}()

				if remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeVP8 || remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeVP9 || remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeH264 {
					// Create a local video track, all our SFU clients will be fed via this track
					var err error
					track, err := pcPub.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "video", "pion")
					checkError(err)

					m.Video = track
					mediaInfo[name] = m

					defer func() {
						checkError(ivfFile.Close())
						checkError(opusFile.Close())
					}()

					for {
						pkt, err := remoteTrack.ReadRTP()
						checkError(err)
						err = track.WriteRTP(pkt)

						codec := remoteTrack.Codec()
						if codec.Name == webrtc.Opus {
							fmt.Println("Got Opus track, saving to disk as output.opus (48 kHz, 2 channels)")
							err := opusFile.WriteRTP(pkt)
							if err != nil {
								checkError(err)
							}
						} else if codec.Name == webrtc.VP8 {
							fmt.Println("Got VP8 track, saving to disk as output.ivf")
							err := ivfFile.WriteRTP(pkt)
							if err != nil {
								checkError(err)
							}
						}

						if err != io.ErrClosedPipe {
							checkError(err)
						}
					}

				} else {
					// Create a local audio track, all our SFU clients will be fed via this track
					var err error
					audioTrack, err := pcPub.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "audio", "pion")
					checkError(err)

					m.Audio = audioTrack
					if _, ok := mediaInfo[name]; !ok {
						mediaInfo[name] = m
					}

					defer func() {
						checkError(opusFile.Close())
					}()
					for {
						pkt, err := remoteTrack.ReadRTP()
						checkError(err)
						err = audioTrack.WriteRTP(pkt)
						if err != io.ErrClosedPipe {
							checkError(err)
						}

						err = opusFile.WriteRTP(pkt)
						checkError(err)
					}
				}
			})

			// Set the remote SessionDescription
			checkError(pcPub.SetRemoteDescription(
				webrtc.SessionDescription{
					SDP:  sdp,
					Type: webrtc.SDPTypeOffer,
				}))

			// Create answer
			answer, err := pcPub.CreateAnswer(nil)
			checkError(err)

			// Sets the LocalDescription, and starts our UDP listeners
			checkError(pcPub.SetLocalDescription(answer))

			// Send server sdp to publisher
			dataToClient := wsMsg{
				Type: "publish",
				Sdp:  answer.SDP,
				Name: name,
			}

			byteToClient, err := json.Marshal(dataToClient)
			checkError(err)

			if err := c.WriteMessage(mt, byteToClient); err != nil {
				checkError(err)
			}

		}

		if wsData.Type == "subscribe" {
			m = getRcvMedia(name, mediaInfo)

			pcSub, err := api.NewPeerConnection(peerConnectionConfig)
			checkError(err)

			if m.Video != nil {
				_, err = pcSub.AddTrack(m.Video)
				checkError(err)
			}
			if m.Audio != nil {
				_, err = pcSub.AddTrack(m.Audio)
				checkError(err)
			}

			checkError(pcSub.SetRemoteDescription(
				webrtc.SessionDescription{
					SDP:  string(sdp),
					Type: webrtc.SDPTypeOffer,
				}))

			answer, err := pcSub.CreateAnswer(nil)
			checkError(err)

			// Sets the LocalDescription, and starts our UDP listeners
			checkError(pcSub.SetLocalDescription(answer))

			// Send sdp
			dataToClient := wsMsg{
				Type: "subscribe",
				Sdp:  answer.SDP,
				Name: name,
			}
			byteToClient, err := json.Marshal(dataToClient)
			checkError(err)

			if err := c.WriteMessage(mt, byteToClient); err != nil {
				checkError(err)
			}
		}
	}
}
