package peerconnectionchannel

import (
	"encoding/json"
	"errors"
	"log/slog"

	sensor_msgs_msg "github.com/3DRX/webrtc-ros-bridge/rclgo_gen/sensor_msgs/msg"
	rosmediadevicesadapter "github.com/3DRX/webrtc-ros-bridge/ros_mediadevices_adapter"
	send_signalingchannel "github.com/3DRX/webrtc-ros-bridge/sender/signaling_channel"
	"github.com/pion/interceptor"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/codec/vpx"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/webrtc/v3"
	"github.com/tiiuae/rclgo/pkg/rclgo"
	"github.com/tiiuae/rclgo/pkg/rclgo/types"
)

type AddStreamAction struct {
	Type string `json:"type"`
	Id   string `json:"id"`
}

type AddVideoTrackAction struct {
	Type     string `json:"type"`
	Id       string `json:"id"`
	StreamId string `json:"stream_id"`
	SrcId    string `json:"src"`
}

type PeerConnectionChannel struct {
	imgChan           <-chan *sensor_msgs_msg.Image
	sensorChan        <-chan types.Message
	chanDispatcher    func()
	sendSDPChan       chan<- webrtc.SessionDescription
	recvSDPChan       <-chan webrtc.SessionDescription
	sendCandidateChan chan<- webrtc.ICECandidateInit
	recvCandidateChan <-chan webrtc.ICECandidateInit
	peerConnection    *webrtc.PeerConnection
}

func InitPeerConnectionChannel(
	messageChan <-chan types.Message,
	sendSDPChan chan<- webrtc.SessionDescription,
	recvSDPChan <-chan webrtc.SessionDescription,
	sendCandidateChan chan<- webrtc.ICECandidateInit,
	recvCandidateChan <-chan webrtc.ICECandidateInit,
	action *send_signalingchannel.Action,
) *PeerConnectionChannel {
	// parse action
	if action.Type != "configure" {
		panic("Invalid action type")
	}
	rawActions := action.Actions
	if len(rawActions) != 2 {
		panic("Invalid number of actions")
	}
	rawAddStream := rawActions[0]
	rawAddVideoTrack := rawActions[1]
	// bind raw actions to struct
	addStreamAction := AddStreamAction{}
	addVideoTrackAction := AddVideoTrackAction{}
	if err := unmarshalAction(rawAddStream, &addStreamAction); err != nil {
		panic(err)
	}
	if err := unmarshalAction(rawAddVideoTrack, &addVideoTrackAction); err != nil {
		panic(err)
	}
	// TODO: read data from action and use the action to select
	// ROS topic to send through bridge.
	// For now, we just send the ROS topic specified in the config.

	// create a dispatch goroutine to split image message from other sensor messages
	imgChan := make(chan *sensor_msgs_msg.Image, 10)
	sensorChan := make(chan types.Message, 10)

	rosmediadevicesadapter.Initialize(imgChan)
	vp8Params, err := vpx.NewVP8Params()
	if err != nil {
		panic(err)
	}
	vp8Params.BitRate = 5_000_000
	codecselector := mediadevices.NewCodecSelector(
		mediadevices.WithVideoEncoders(&vp8Params),
	)
	m := &webrtc.MediaEngine{}
	codecselector.Populate(m)
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
	slog.Info("Created peer connection")

	mediaStream, err := mediadevices.GetUserMedia(mediadevices.MediaStreamConstraints{
		Video: func(constraint *mediadevices.MediaTrackConstraints) {
			constraint.Width = prop.Int(640)
			constraint.Height = prop.Int(480)
		},
		Codec: codecselector,
	})
	if err != nil {
		panic(err)
	}
	for _, videoTrack := range mediaStream.GetVideoTracks() {
		videoTrack.OnEnded(func(err error) {
			slog.Error("Track ended", "error", err)
		})
		_, err := peerConnection.AddTransceiverFromTrack(
			videoTrack,
			webrtc.RtpTransceiverInit{
				Direction: webrtc.RTPTransceiverDirectionSendonly,
			},
		)
		if err != nil {
			panic(err)
		}
		slog.Info("add video track success")
	}

	pc := &PeerConnectionChannel{
		sendSDPChan:       sendSDPChan,
		recvSDPChan:       recvSDPChan,
		sendCandidateChan: sendCandidateChan,
		recvCandidateChan: recvCandidateChan,
		peerConnection:    peerConnection,
		imgChan:           imgChan,
		sensorChan:        sensorChan,
		chanDispatcher: func() {
			for {
				msg := <-messageChan
				switch msg.(type) {
				case *sensor_msgs_msg.Image:
					imgChan <- msg.(*sensor_msgs_msg.Image)
				default:
					sensorChan <- msg
				}
			}
		},
	}
	return pc
}

func (pc *PeerConnectionChannel) handleRemoteICECandidate() {
	for {
		candidate := <-pc.recvCandidateChan
		if err := pc.peerConnection.AddICECandidate(candidate); err != nil {
			panic(err)
		}
	}
}

func (pc *PeerConnectionChannel) Spin() {
	go pc.chanDispatcher()

	datachannel, err := pc.peerConnection.CreateDataChannel("data", nil)
	if err != nil {
		panic(err)
	}
	datachannel.OnOpen(func() {
		slog.Info("datachannel open", "label", datachannel.Label(), "ID", datachannel.ID())
		for {
			sensorMsg := <-pc.sensorChan
			serializedMsg, err := rclgo.Serialize(sensorMsg)
			if err != nil {
				slog.Error("failed to serialize sensor message", "error", err)
				continue
			}
			datachannel.Send(serializedMsg)
		}
	})
	datachannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		slog.Info("datachannel message", "data", string(msg.Data))
	})

	offer, err := pc.peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}
	pc.peerConnection.SetLocalDescription(offer)
	pc.peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		pc.sendCandidateChan <- c.ToJSON()
	})
	go pc.handleRemoteICECandidate()
	pc.sendSDPChan <- offer
	remoteSDP := <-pc.recvSDPChan
	pc.peerConnection.SetRemoteDescription(remoteSDP)
}

func unmarshalAction(rawAction interface{}, action interface{}) error {
	rawActionMap, ok := rawAction.(map[string]interface{})
	if !ok {
		return errors.New("Invalid action type")
	}
	rawActionBytes, err := json.Marshal(rawActionMap)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(rawActionBytes, action); err != nil {
		return err
	}
	return nil
}
