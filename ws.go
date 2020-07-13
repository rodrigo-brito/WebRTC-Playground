package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v2"
	"github.com/pion/webrtc/v2/pkg/media/ivfwriter"
	"github.com/pion/webrtc/v2/pkg/media/opuswriter"
	log "github.com/sirupsen/logrus"
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
	// Publisher Peer
	pcPub *webrtc.PeerConnection

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

type wsError struct {
	Type    string
	Message string
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

type SFU struct {
	// Media engine
	m webrtc.MediaEngine

	// API object
	api *webrtc.API
}

func NewSFU() SFU {
	var sfu SFU
	// Create a MediaEngine object to configure the supported codec
	sfu.m = webrtc.MediaEngine{}

	settingEngine := webrtc.SettingEngine{}
	settingEngine.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})

	// Setup the codecs you want to use.
	sfu.m.RegisterDefaultCodecs()

	// Create the API object with the MediaEngine
	sfu.api = webrtc.NewAPI(
		webrtc.WithMediaEngine(sfu.m),
		webrtc.WithSettingEngine(settingEngine),
	)

	return sfu
}

func ResponseWSError(conn *websocket.Conn, id int, err error) {
	errMessage := wsError{
		Type:    "error",
		Message: err.Error(),
	}

	byteToClient, err := json.Marshal(errMessage)
	if err != nil {
		log.Error(err)
		return
	}

	if err := conn.WriteMessage(id, byteToClient); err != nil {
		log.Error(err)
	}
}

func (s *SFU) ws(w http.ResponseWriter, r *http.Request) {
	// Websocket client
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	defer func() {
		err := c.Close()
		if err != nil {
			log.Error(err)
		}
	}()

	for {
		// Read sdp from websocket
		mt, msg, err := c.ReadMessage()
		if err != nil {
			log.Error(err)
			continue
		}

		wsData := wsMsg{}
		if err := json.Unmarshal(msg, &wsData); err != nil {
			log.Error(err)
			continue
		}

		sdp := wsData.Sdp
		name := wsData.Name

		m, ok := mediaInfo[name]

		if !ok {
			mediaInfo[name] = avTrack{}
			m = mediaInfo[name]
		}

		if wsData.Type == "publish" {
			// Create a new RTCPeerConnection
			pcPub, err = s.api.NewPeerConnection(peerConnectionConfig)
			if err != nil {
				log.Error(err)
				continue
			}

			_, err = pcPub.AddTransceiver(webrtc.RTPCodecTypeAudio)
			if err != nil {
				ResponseWSError(c, mt, err)
				return
			}

			_, err = pcPub.AddTransceiver(webrtc.RTPCodecTypeVideo)
			if err != nil {
				ResponseWSError(c, mt, err)
				return
			}

			opusFile, err := opuswriter.New(fmt.Sprintf("%s.ogg", name), 48000, 2)
			if err != nil {
				ResponseWSError(c, mt, err)
				return
			}
			ivfFile, err := ivfwriter.New(fmt.Sprintf("%s.ivf", name))
			if err != nil {
				ResponseWSError(c, mt, err)
				return
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
				}
			})

			// receive av data from chrome
			pcPub.OnTrack(func(remoteTrack *webrtc.Track, receiver *webrtc.RTPReceiver) {
				go func() {
					ticker := time.NewTicker(rtcpPLIInterval)
					for range ticker.C {
						if rtcpSendErr := pcPub.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: remoteTrack.SSRC()}}); rtcpSendErr != nil {
							if err != nil {
								log.Error(err)
							}
						}
					}
				}()

				if remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeVP8 || remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeVP9 || remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeH264 {
					// Create a local video track, all our SFU clients will be fed via this track
					var err error
					track, err := pcPub.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "video", "pion")
					if err != nil {
						log.Error(err)
						return
					}

					m.Video = track
					mediaInfo[name] = m

					defer func() {
						err := ivfFile.Close()
						if err != nil {
							log.Error(err)
						}

						err = opusFile.Close()
						if err != nil {
							log.Error(err)
						}
					}()

					for {
						pkt, err := remoteTrack.ReadRTP()
						if err != nil {
							log.Error(err)
							return
						}

						err = track.WriteRTP(pkt)

						codec := remoteTrack.Codec()
						if codec.Name == webrtc.Opus {
							fmt.Println("Got Opus track, saving to disk as output.opus (48 kHz, 2 channels)")
							err := opusFile.WriteRTP(pkt)
							if err != nil {
								log.Error(err)
								return
							}
						} else if codec.Name == webrtc.VP8 {
							fmt.Println("Got VP8 track, saving to disk as output.ivf")
							err := ivfFile.WriteRTP(pkt)
							if err != nil {
								log.Error(err)
								return
							}
						}

						if err != io.ErrClosedPipe {
							log.Error(err)
							return
						}
					}

				} else {
					// Create a local audio track, all our SFU clients will be fed via this track
					var err error
					audioTrack, err := pcPub.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "audio", "pion")
					if err != nil {
						ResponseWSError(c, mt, err)
						return
					}

					m.Audio = audioTrack
					if _, ok := mediaInfo[name]; !ok {
						mediaInfo[name] = m
					}

					defer func() {
						err := opusFile.Close()
						if err != nil {
							log.Error(err)
						}
					}()
					for {
						pkt, err := remoteTrack.ReadRTP()
						if err != nil {
							ResponseWSError(c, mt, err)
							return
						}

						err = audioTrack.WriteRTP(pkt)
						if err != io.ErrClosedPipe {
							log.Error(err)
							return
						}

						err = opusFile.WriteRTP(pkt)
						if err != nil {
							log.Error(err)
							return
						}
					}
				}
			})

			// Set the remote SessionDescription
			err = pcPub.SetRemoteDescription(
				webrtc.SessionDescription{
					SDP:  sdp,
					Type: webrtc.SDPTypeOffer,
				})
			if err != nil {
				ResponseWSError(c, mt, err)
				return
			}

			// Create answer
			answer, err := pcPub.CreateAnswer(nil)
			if err != nil {
				ResponseWSError(c, mt, err)
				return
			}

			// Sets the LocalDescription, and starts our UDP listeners
			err = pcPub.SetLocalDescription(answer)
			if err != nil {
				ResponseWSError(c, mt, err)
				return
			}

			// Send server sdp to publisher
			dataToClient := wsMsg{
				Type: "publish",
				Sdp:  answer.SDP,
				Name: name,
			}

			byteToClient, err := json.Marshal(dataToClient)
			if err != nil {
				ResponseWSError(c, mt, err)
				return
			}

			if err := c.WriteMessage(mt, byteToClient); err != nil {
				log.Error(err)
			}
		}

		if wsData.Type == "subscribe" {
			m = getRcvMedia(name, mediaInfo)

			pcSub, err := s.api.NewPeerConnection(peerConnectionConfig)
			if err != nil {
				ResponseWSError(c, mt, err)
				return
			}

			if m.Video != nil {
				_, err = pcSub.AddTrack(m.Video)
				if err != nil {
					ResponseWSError(c, mt, err)
					return
				}
			}
			if m.Audio != nil {
				_, err = pcSub.AddTrack(m.Audio)
				if err != nil {
					ResponseWSError(c, mt, err)
					return
				}
			}

			err = pcSub.SetRemoteDescription(
				webrtc.SessionDescription{
					SDP:  string(sdp),
					Type: webrtc.SDPTypeOffer,
				})
			if err != nil {
				ResponseWSError(c, mt, err)
				return
			}

			answer, err := pcSub.CreateAnswer(nil)
			if err != nil {
				ResponseWSError(c, mt, err)
				return
			}

			// Sets the LocalDescription, and starts our UDP listeners
			err = pcSub.SetLocalDescription(answer)
			if err != nil {
				ResponseWSError(c, mt, err)
				return
			}

			// Send sdp
			dataToClient := wsMsg{
				Type: "subscribe",
				Sdp:  answer.SDP,
				Name: name,
			}
			byteToClient, err := json.Marshal(dataToClient)
			if err != nil {
				ResponseWSError(c, mt, err)
				return
			}

			if err := c.WriteMessage(mt, byteToClient); err != nil {
				log.Error(err)
			}
		}
	}
}
