package main

import (
	"github.com/3DRX/webrtc-ros-bridge/config"
	recv_peerconnectionchannel "github.com/3DRX/webrtc-ros-bridge/receiver/peer_connection_channel"
	recv_roschannel "github.com/3DRX/webrtc-ros-bridge/receiver/ros_channel"
	recv_signalingchannel "github.com/3DRX/webrtc-ros-bridge/receiver/signaling_channel"
	send_peerconnectionchannel "github.com/3DRX/webrtc-ros-bridge/sender/peer_connection_channel"
	send_roschannel "github.com/3DRX/webrtc-ros-bridge/sender/ros_channel"
	send_signalingchannel "github.com/3DRX/webrtc-ros-bridge/sender/signaling_channel"
	"github.com/pion/webrtc/v3"
	"github.com/tiiuae/rclgo/pkg/rclgo/types"
)

func receiver(cfg *config.Config) {
	messageChan := make(chan types.Message)
	sdpChan := make(chan webrtc.SessionDescription)
	sdpReplyChan := make(chan webrtc.SessionDescription)
	candidateChan := make(chan webrtc.ICECandidateInit)
	topicIdx := 0 // TODO: refactor similar to sender
	sc := recv_signalingchannel.InitSignalingChannel(
		cfg,
		topicIdx,
		sdpChan,
		sdpReplyChan,
		candidateChan,
	)
	pc := recv_peerconnectionchannel.InitPeerConnectionChannel(
		sdpChan,
		sdpReplyChan,
		candidateChan,
		sc.SignalCandidate,
		messageChan,
	)
	rc := recv_roschannel.InitROSChannel(
		cfg,
		topicIdx,
		messageChan,
	)
	go sc.Spin()
	go pc.Spin()
	go rc.Spin()
	select {}
}

func sender(cfg *config.Config) {
	messageChan := make(chan types.Message)
	sendSDPChan := make(chan webrtc.SessionDescription)
	recvSDPChan := make(chan webrtc.SessionDescription)
	sendCandidateChan := make(chan webrtc.ICECandidateInit)
	recvCandidateChan := make(chan webrtc.ICECandidateInit)
	sc := send_signalingchannel.InitSignalingChannel(
		cfg,
		sendSDPChan,
		recvSDPChan,
		sendCandidateChan,
		recvCandidateChan,
	)
	haveReceiverPromise := sc.Spin()
	<-haveReceiverPromise
	actions := sc.GetActions()
	rc := send_roschannel.InitROSChannel(
		cfg,
		messageChan,
	)
	go rc.Spin()
	pc := send_peerconnectionchannel.InitPeerConnectionChannel(
		messageChan,
		sendSDPChan,
		recvSDPChan,
		sendCandidateChan,
		recvCandidateChan,
		actions,
	)
	go pc.Spin()
	select {}
}

func main() {
	cfg := config.LoadCfg()
	if cfg.Mode == "receiver" {
		receiver(cfg)
	} else if cfg.Mode == "sender" {
		sender(cfg)
	} else {
		panic("unsupported mode")
	}
}
