package roschannel

import (
	"context"
	"log/slog"

	"github.com/3DRX/webrtc-ros-bridge/config"
	"github.com/3DRX/webrtc-ros-bridge/consts"
	sensor_msgs_msg "github.com/3DRX/webrtc-ros-bridge/rclgo_gen/sensor_msgs/msg"
	"github.com/tiiuae/rclgo/pkg/rclgo"
	"github.com/tiiuae/rclgo/pkg/rclgo/types"
)

type ROSChannel struct {
	subscriptions []*rclgo.Subscription
	node          *rclgo.Node
}

func InitROSChannel(
	cfg *config.Config,
	messageChan chan<- types.Message,
) *ROSChannel {
	nodeName := "webrtc_ros_bridge_" + cfg.Mode
	slog.Info("creating node", "name", nodeName)
	err := rclgo.Init(nil)
	if err != nil {
		panic(err)
	}
	node, err := rclgo.NewNode(nodeName, "")
	if err != nil {
		panic(err)
	}
	// create subscriptions based on topic types
	subs := make([]*rclgo.Subscription, len(cfg.Topics))
	for i, topic := range cfg.Topics {
		switch topic.Type {
		case consts.MSG_IMAGE:
			imgSub, err := sensor_msgs_msg.NewImageSubscription(
				node,
				"/"+cfg.Topics[i].NameIn,
				nil,
				func(msg *sensor_msgs_msg.Image, info *rclgo.MessageInfo, err error) {
					messageChan <- msg
				},
			)
			subs[i] = imgSub.Subscription
			if err != nil {
				panic(err)
			}
		case consts.MSG_LASER_SCAN:
			laserScanSub, err := sensor_msgs_msg.NewLaserScanSubscription(
				node,
				"/"+cfg.Topics[i].NameIn,
				nil,
				func(msg *sensor_msgs_msg.LaserScan, info *rclgo.MessageInfo, err error) {
					messageChan <- msg
				},
			)
			subs[i] = laserScanSub.Subscription
			if err != nil {
				panic(err)
			}
		default:
			panic("unsupported topic type")
		}
	}
	return &ROSChannel{
		subscriptions: subs,
		node:          node,
	}
}

func (r *ROSChannel) Spin() {
	defer r.node.Close()
	defer func() {
		for _, sub := range r.subscriptions {
			sub.Close()
		}
	}()
	ws, err := rclgo.NewWaitSet()
	if err != nil {
		panic(err)
	}
	defer ws.Close()
	ws.AddSubscriptions(r.subscriptions...)
	ws.Run(context.Background())
}
