package roschannel

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/3DRX/webrtc-ros-bridge/config"
	sensor_msgs_msg "github.com/3DRX/webrtc-ros-bridge/rclgo_gen/sensor_msgs/msg"
	"github.com/tiiuae/rclgo/pkg/rclgo"
	"github.com/tiiuae/rclgo/pkg/rclgo/types"
)

type ROSChannel struct {
	chanDispatcher func()
	imgChan        <-chan *sensor_msgs_msg.Image
	sensorChan     <-chan types.Message
	cfg            *config.Config
	topicIdx       int
}

func InitROSChannel(
	cfg *config.Config,
	topicIdx int,
	messageChan <-chan types.Message,
) *ROSChannel {
	imgChan := make(chan *sensor_msgs_msg.Image, 10)
	sensorChan := make(chan types.Message, 10)
	return &ROSChannel{
		cfg:        cfg,
		topicIdx:   topicIdx,
		imgChan:    imgChan,
		sensorChan: sensorChan,
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
}

func (r *ROSChannel) Spin() {
	go r.chanDispatcher()

	err := rclgo.Init(nil)
	if err != nil {
		panic(err)
	}
	nodeName := "webrtc_ros_bridge_" + r.cfg.Mode + "_" + r.cfg.Topics[r.topicIdx].Type + "_" + r.cfg.Topics[r.topicIdx].NameOut
	nodeName = strings.ReplaceAll(nodeName, "/", "_")
	node, err := rclgo.NewNode(nodeName, "")
	if err != nil {
		panic(err)
	}
	defer node.Close()
	pub, err := sensor_msgs_msg.NewImagePublisher(node, "/"+r.cfg.Topics[r.topicIdx].NameOut, nil)
	if err != nil {
		panic(err)
	}
	defer pub.Close()
	defer rclgo.Uninit()

	const windowSize = 30
	timestamps := make([]time.Time, windowSize)
	frameCount := 0
	idx := 0
	firstFrame := true
	lastPrintTime := time.Now()

	for {
		img := <-r.imgChan
		now := time.Now()
		err := pub.Publish(img)
		if err != nil {
			slog.Error("Failed to publish image message", "error", err)
		}

		if firstFrame {
			timestamps[0] = now
			firstFrame = false
			frameCount = 1
			continue
		}
		idx = (idx + 1) % windowSize
		timestamps[idx] = now
		if frameCount < windowSize {
			frameCount++
		}
		if now.Sub(lastPrintTime) >= time.Second {
			if frameCount < 2 {
				slog.Info("FPS calculation pending, need more frames")
				continue
			}
			oldestIdx := (idx - frameCount + 1 + windowSize) % windowSize
			duration := timestamps[idx].Sub(timestamps[oldestIdx])
			if duration.Seconds() > 0 {
				fps := float64(frameCount-1) / duration.Seconds()
				slog.Info("Current FPS", "fps", fmt.Sprintf("%.2f", fps))
			}
			lastPrintTime = now
		}
	}
}
