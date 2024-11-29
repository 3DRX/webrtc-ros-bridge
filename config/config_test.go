package config

import (
	"testing"
)

func TestCheckfunc(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected bool
	}{
		{
			name: "valid config",
			cfg: &Config{
				Mode: "receiver",
				Addr: "10.3.9.3:8080",
				Topics: []TopicConfig{
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
			name: "valid config with hostname",
			cfg: &Config{
				Mode: "receiver",
				Addr: "localhost:8080",
				Topics: []TopicConfig{
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
			cfg: &Config{
				Mode: "",
				Addr: "",
				Topics: []TopicConfig{
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
			err := checkCfg(tt.cfg)
			msg := true
			if err != nil {
				msg = false
			}
			if msg != tt.expected {
				t.Errorf("TestCheckfunc failed: expected status %v, got %v", tt.expected, err.Error())
			}
		})
	}
}
