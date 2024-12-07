package peerconnectionchannel

import (
	"fmt"
	"log/slog"
	"time"

	sensor_msgs_msg "github.com/3DRX/webrtc-ros-bridge/rclgo_gen/sensor_msgs/msg"
	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"github.com/tiiuae/rclgo/pkg/rclgo"
	"github.com/tiiuae/rclgo/pkg/rclgo/types"
)

type PeerConnectionChannel struct {
	sdpChan         <-chan webrtc.SessionDescription
	sdpReplyChan    chan<- webrtc.SessionDescription
	candidateChan   <-chan webrtc.ICECandidateInit
	peerConnection  *webrtc.PeerConnection
	signalCandidate func(c webrtc.ICECandidateInit) error
	messageChan     chan<- types.Message
}

func registerHeaderExtensionURI(m *webrtc.MediaEngine, uris []string) {
	for _, uri := range uris {
		err := m.RegisterHeaderExtension(
			webrtc.RTPHeaderExtensionCapability{
				URI: uri,
			},
			webrtc.RTPCodecTypeVideo,
			webrtc.RTPTransceiverDirectionRecvonly,
		)
		if err != nil {
			panic(err)
		}
	}
}

func InitPeerConnectionChannel(
	sdpChan chan webrtc.SessionDescription,
	sdpReplyChan chan<- webrtc.SessionDescription,
	candidateChan <-chan webrtc.ICECandidateInit,
	signalCandidate func(c webrtc.ICECandidateInit) error,
	messageChan chan<- types.Message,
) *PeerConnectionChannel {
	m := &webrtc.MediaEngine{}
	// Register VP8
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000, Channels: 0},
		PayloadType:        96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}

	registerHeaderExtensionURI(m, []string{
		"urn:ietf:params:rtp-hdrext:toffset",
		"http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		"urn:3gpp:video-orientation",
		"http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
		"http://www.webrtc.org/experiments/rtp-hdrext/playout-delay",
		"http://www.webrtc.org/experiments/rtp-hdrext/video-content-type",
		"http://www.webrtc.org/experiments/rtp-hdrext/video-timing",
		"http://www.webrtc.org/experiments/rtp-hdrext/color-space",
	})

	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		panic(err)
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}
	return &PeerConnectionChannel{
		sdpChan:         sdpChan,
		sdpReplyChan:    sdpReplyChan,
		candidateChan:   candidateChan,
		peerConnection:  peerConnection,
		signalCandidate: signalCandidate,
		messageChan:     messageChan,
	}
}

func handleSignalingMessage(pc *PeerConnectionChannel) {
	for {
		select {
		case sdp := <-pc.sdpChan:
			slog.Info("received SDP", "sdp", sdp.SDP)
			err := pc.peerConnection.SetRemoteDescription(sdp)
			if err != nil {
				panic(err)
			}
			answer, err := pc.peerConnection.CreateAnswer(nil)
			if err != nil {
				panic(err)
			}
			pc.sdpReplyChan <- answer
			err = pc.peerConnection.SetLocalDescription(answer)
			if err != nil {
				panic(err)
			}
		case candidate := <-pc.candidateChan:
			err := pc.peerConnection.AddICECandidate(candidate)
			if err != nil {
				panic(err)
			}
			slog.Info("received ICE candidate", "candidate", candidate)
		}
	}
}

func (pc *PeerConnectionChannel) Spin() {
	webmSaver := newWebmSaver(pc.messageChan)
	_, err := pc.peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo,
		webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionRecvonly,
		},
	)
	if err != nil {
		panic(err)
	}
	pc.peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		if err := pc.signalCandidate(c.ToJSON()); err != nil {
			panic(err)
		}
	})
	pc.peerConnection.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		slog.Info("PeerConnectionChannel: received track", "track", track.ID())
		if track.Kind() == webrtc.RTPCodecTypeVideo {
			// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
			go func() {
				ticker := time.NewTicker(time.Second * 3)
				for range ticker.C {
					errSend := pc.peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())}})
					if errSend != nil {
						fmt.Println(errSend)
					}
				}
			}()
		}
		for {
			rtp, _, readErr := track.ReadRTP()
			if readErr != nil {
				panic(readErr)
			}
			webmSaver.PushVP8(rtp)
		}
	})
	pc.peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			serializedMsg := msg.Data
			sensorMsg, err := rclgo.Deserialize(serializedMsg, sensor_msgs_msg.LaserScanTypeSupport)
			if err != nil {
				slog.Error("failed to deserialize sensor message", "error", err)
				return
			}
			laserScanMsg, ok := sensorMsg.(*sensor_msgs_msg.LaserScan)
			if !ok {
				slog.Error("failed to cast sensor message to LaserScan")
				return
			}
			slog.Info("datachannel message", "frame ID", laserScanMsg.Header.FrameId)
		})
		d.OnOpen(func() {
			slog.Info("datachannel open", "label", d.Label(), "ID", d.ID())
			d.SendText("Hello from receiver!")
		})
	})
	go handleSignalingMessage(pc)
}
