package main

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	log "github.com/sirupsen/logrus"
)

type CommandType string

const (
	Offer      CommandType = "offer"
	Answer     CommandType = "answer"
	Connect    CommandType = "connect"
	Disconnect CommandType = "disconnect"
)

type Payload struct {
	From    string      `json:"from"`
	To      string      `json:"to"`
	Data    string      `json:"data"`
	Command CommandType `json:"command"`
}

type Server struct {
	users map[string]net.Conn
}

func NewServer() *Server {
	return &Server{
		users: make(map[string]net.Conn),
	}
}

func (s *Server) writerWS(conn net.Conn, code ws.OpCode, payload Payload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	err = wsutil.WriteServerMessage(conn, code, data)
	if err != nil {
		conn.Close()
		delete(s.users, payload.To)
		return err
	}

	return nil
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	go func() {
		defer conn.Close()

		for {
			data, op, err := wsutil.ReadClientData(conn)
			if err != nil {
				if !errors.As(err, &wsutil.ClosedError{}) {
					log.Errorf("read: %s, %d", err, op)
				}
				return
			}

			var payload Payload
			err = json.Unmarshal(data, &payload)
			if err != nil {
				log.Errorf("unmarshal: %s", err)
				continue
			}

			if op == ws.OpClose {
				return
			}

			switch payload.Command {
			case Offer:
				log.Infof("offer %s -> %s", payload.From, payload.To)
				err = s.writerWS(conn, op, payload)
				if err != nil {
					log.Errorf("write connect: %s", err)
				}

			case Answer:
				log.Infof("answer %s -> %s", payload.From, payload.To)
				err = s.writerWS(conn, op, payload)
				if err != nil {
					log.Errorf("write connect: %s", err)
				}

			case Connect:
				s.users[payload.From] = conn
				log.Info("connecting: ", payload.From)
				// notify all users a new connection
				for id, conn := range s.users {
					if id == payload.From {
						continue
					}

					err = s.writerWS(conn, op, Payload{
						To:      id,
						From:    payload.From,
						Command: Connect,
					})
					if err != nil {
						log.Errorf("write connect: %s", err)
					}
				}

			case Disconnect:
				delete(s.users, payload.From)
				log.Info("disconnecting: ", payload.From)
				// notify all users a peer disconnection
				for id, conn := range s.users {
					if id == payload.From {
						continue
					}

					err = s.writerWS(conn, op, Payload{
						To:      id,
						From:    payload.From,
						Command: Disconnect,
					})
					if err != nil {
						log.Errorf("write disconnect: %s", err)
					}
				}
			}
		}
	}()
}
