package peerconnectionchannel

/*
#cgo LDFLAGS: -L. -lvp8decoder -lvpx -lm
#include "vp8_decoder.h"
*/
import "C"

import (
	"log/slog"
	"time"
	"unsafe"

	sensor_msgs_msg "github.com/3DRX/webrtc-ros-bridge/rclgo_gen/sensor_msgs/msg"
	"github.com/pion/interceptor/pkg/jitterbuffer"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v3/pkg/media/samplebuilder"
	"github.com/tiiuae/rclgo/pkg/rclgo/types"
)

type WebmSaver struct {
	vp8Builder     *samplebuilder.SampleBuilder
	videoTimestamp time.Duration

	h264JitterBuffer   *jitterbuffer.JitterBuffer
	lastVideoTimestamp uint32
	codecCtx           C.vpx_codec_ctx_t
	codecCreated       bool
	imgChan            chan<- types.Message
}

func newWebmSaver(imgChan chan<- types.Message) *WebmSaver {
	return &WebmSaver{
		vp8Builder:       samplebuilder.New(200, &codecs.VP8Packet{}, 90000),
		h264JitterBuffer: jitterbuffer.New(),
		imgChan:          imgChan,
		codecCreated:     false,
	}
}

func (s *WebmSaver) Close() {
	if s.codecCreated {
		// TODO close codec
	}
}

func (s *WebmSaver) PushVP8(rtpPacket *rtp.Packet) {
	s.vp8Builder.Push(rtpPacket)

	for {
		sample := s.vp8Builder.Pop()
		if sample == nil {
			return
		}
		// Read VP8 header.
		videoKeyframe := (sample.Data[0]&0x1 == 0)
		if videoKeyframe {
			// Keyframe has frame information.
			raw := uint(sample.Data[6]) | uint(sample.Data[7])<<8 | uint(sample.Data[8])<<16 | uint(sample.Data[9])<<24
			width := int(raw & 0x3FFF)
			height := int((raw >> 16) & 0x3FFF)

			if !s.codecCreated {
				s.InitWriter(width, height)
			}
		}

		// Decode VP8 frame
		codecError := C.decode_frame(&s.codecCtx, (*C.uint8_t)(&sample.Data[0]), C.size_t(len(sample.Data)))
		if codecError != 0 {
			slog.Error("Decode error", "errorCode", codecError)
			continue
		}
		// Get decoded frames
		var iter C.vpx_codec_iter_t
		img := C.vpx_codec_get_frame(&s.codecCtx, &iter)
		if img == nil {
			slog.Error("Failed to get decoded frame")
			continue
		}
		var ros_img sensor_msgs_msg.Image
		var ros_img_c C.sensor_msgs__msg__Image
		C.vpx_to_ros_image(img, &ros_img_c)
		sensor_msgs_msg.ImageTypeSupport.AsGoStruct(&ros_img, unsafe.Pointer(&ros_img_c))
		C.cleanup_ros_image(&ros_img_c)
		s.imgChan <- &ros_img
	}
}

func (s *WebmSaver) InitWriter(width, height int) {
	if errCode := C.init_decoder(&s.codecCtx, C.uint(width), C.uint(height)); errCode != 0 {
		slog.Error("failed to initialize decoder", "error", errCode)
	}
	s.codecCreated = true
}
