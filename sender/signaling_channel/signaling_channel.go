package signalingchannel

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/3DRX/webrtc-ros-bridge/config"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

type Action struct {
	Type    string                   `json:"type"`
	Actions []map[string]interface{} `json:"actions"`
}

type SignalingChannel struct {
	cfg                 *config.Config
	upgrader            *websocket.Upgrader
	conn                *websocket.Conn
	actions             *Action
	haveReceiverPromise chan struct{}
	sendSDPChan         <-chan webrtc.SessionDescription
	recvSDPChan         chan<- webrtc.SessionDescription
	sendCandidateChan   <-chan webrtc.ICECandidateInit
	recvCandidateChan   chan<- webrtc.ICECandidateInit
}

func InitSignalingChannel(
	cfg *config.Config,
	sendSDPChan <-chan webrtc.SessionDescription,
	recvSDPChan chan<- webrtc.SessionDescription,
	sendCandidateChan <-chan webrtc.ICECandidateInit,
	recvCandidateChan chan<- webrtc.ICECandidateInit,
) *SignalingChannel {
	return &SignalingChannel{
		cfg: cfg,
		upgrader: &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		conn:                nil,
		actions:             nil,
		haveReceiverPromise: make(chan struct{}),
		sendSDPChan:         sendSDPChan,
		recvSDPChan:         recvSDPChan,
		sendCandidateChan:   sendCandidateChan,
		recvCandidateChan:   recvCandidateChan,
	}
}

func (s *SignalingChannel) Spin() <-chan struct{} {
	mux := http.NewServeMux()
	mux.Handle("GET /webrtc", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.conn != nil {
			slog.Warn("already have a receiver, rejecting new connection")
			w.WriteHeader(http.StatusConflict)
			return
		}
		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			panic(err)
		}
		slog.Info("new receiver connected")
		s.conn = conn
		go s.handleRecvMessages()
		go s.handleSendMessages()
	}))

	httpServer := &http.Server{
		Addr:    s.cfg.Addr,
		Handler: mux,
	}
	go func() {
		err := httpServer.ListenAndServe()
		if err != nil {
			panic(err)
		}
	}()
	return s.haveReceiverPromise
}

func (s *SignalingChannel) handleSendMessages() {
	for {
		select {
		case sdp := <-s.sendSDPChan:
			jsonMsg, err := json.Marshal(sdp)
			if err != nil {
				slog.Error("failed to marshal SDP", "error", err)
			}
			err = s.conn.WriteMessage(websocket.TextMessage, jsonMsg)
			if err != nil {
				slog.Error("websocket write error", "error", err)
			}
			slog.Info("sent SDP", "sdp", sdp.SDP)
		case candidate := <-s.sendCandidateChan:
			jsonMsg, err := json.Marshal(candidate)
			if err != nil {
				slog.Error("failed to marshal ICE candidate", "error", err)
			}
			err = s.conn.WriteMessage(websocket.TextMessage, jsonMsg)
			if err != nil {
				slog.Error("websocket write error", "error", err)
			}
			slog.Info("sent ICE candidate", "candidate", candidate)
		}
	}
}

func (s *SignalingChannel) handleRecvMessages() {
	for {
		_, message, err := s.conn.ReadMessage()
		if err != nil {
			slog.Error("websocket read error", "error", err)
			return
		}
		if s.actions == nil {
			// try to parse the message as an action
			newAction := &Action{}
			err := json.Unmarshal(message, newAction)
			if err != nil {
				slog.Warn("failed to parse message as action", "error", err)
				continue
			}
			slog.Info("received action", "action", newAction)
			s.actions = newAction
			s.haveReceiverPromise <- struct{}{}
			_, message, err = s.conn.ReadMessage()
			if err != nil {
				slog.Error("websocket read error", "error", err)
				return
			}
			// try to parse it as an SDP
			newSDP := webrtc.SessionDescription{}
			err = json.Unmarshal(message, &newSDP)
			if err != nil {
				slog.Error("failed to parse message as SDP", "error", err)
				continue
			}
			slog.Info("received SDP", "sdp", newSDP.SDP)
			s.recvSDPChan <- newSDP
		}
	}
}

func (s *SignalingChannel) GetActions() *Action {
	return s.actions
}
