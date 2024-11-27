package config_test

import (
	"testing"
	"github.com/3DRX/webrtc-ros-bridge/config"
)

func TestCheckfunc(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		expected bool
	}{
		{
			name: "valid config",
			cfg: &config.Config{
				Mode: "receiver",
				Addr: "10.3.9.3:8080",
				Topics: []config.TopicConfig{
					{
						NameIn:  "image_raw",
						NameOut: "image",
						Type:    "sensor_msgs/msg/Image",
					},
					{
						NameIn:  "image_raw",
						NameOut: "image",
						Type:    "sensor_msgs/msg/Image",
					},
				},
			},
			expected: true,
		},
		{
			name: "invalid config",
			cfg: &config.Config{
				Mode: "",
				Addr: "",
				Topics: []config.TopicConfig{
					{
						NameIn:  "image_raw",
						NameOut: "image",
						Type:    "sensor_msgs/msg/Image",
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := config.CheckCfg(tt.cfg)
			if msg.Status != tt.expected {
				t.Errorf("TestCheckfunc failed: expected status %v, got %v", tt.expected, msg.Status)
			}
		})
	}
}
