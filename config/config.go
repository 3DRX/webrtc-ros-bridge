package config

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"regexp"
	"strings"
)

type TopicConfig struct {
	NameIn  string `json:"name_in"`
	NameOut string `json:"name_out"`
	Type    string `json:"type"` // only "sensor_msgs/msg/Image" is supported
}

type Config struct {
	Mode   string        `json:"mode"` // either "sender" or "receiver"
	Addr   string        `json:"addr"` // http service address
	Topics []TopicConfig `json:"topics"`
}

func isTopicNameValid(topic_name *string) bool {
	re := regexp.MustCompile(`^[a-z0-9_\-]+(/[a-z0-9_\-]+)*$`)
	if *topic_name == "" {
		return false
	}
	return re.MatchString(*topic_name)
}

func isValidIp(addr *string) bool {
	// Try to separate hostname and port
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		// If splitting fails, assume the entire string is a host
		host = *addr
	}

	// First try to parse as IP address
	ip := net.ParseIP(host)
	if ip != nil {
		// If it's a valid IP, check if it's IPv4
		return ip.To4() != nil
	}

	// Check if it's a valid hostname
	if isValidHostname(host) {
		return true
	}

	// If not a valid hostname, try to resolve it
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return false
	}

	// Check if any of the resolved IPs is IPv4
	for _, ip := range ips {
		if ip.To4() != nil {
			return true
		}
	}

	return false
}

func isValidHostname(host string) bool {
	// RFC 1123 hostname validation
	if len(host) > 255 {
		return false
	}
	// Host should not start or end with a dot
	if host[0] == '.' || host[len(host)-1] == '.' {
		return false
	}
	// Split hostname into labels
	labels := strings.Split(host, ".")
	// A valid hostname must have at least one label
	if len(labels) < 1 {
		return false
	}
	for _, label := range labels {
		if len(label) < 1 || len(label) > 63 {
			return false
		}
		// RFC 1123 allows hostname labels to start with a digit
		// Only allow alphanumeric characters and hyphens
		for i, c := range label {
			if !((c >= 'a' && c <= 'z') ||
				(c >= 'A' && c <= 'Z') ||
				(c >= '0' && c <= '9') ||
				(c == '-' && i > 0 && i < len(label)-1)) { // hyphen cannot be first or last
				return false
			}
		}
	}
	return true
}

func checkCfg(c *Config) error {
	if c == nil {
		return fmt.Errorf("get a null pointer!")
	}
	res := true
	// Mode
	res = res && (c.Mode == "sender" || c.Mode == "receiver")
	if res == false {
		return fmt.Errorf("wrong Mode syntax, expected \"sender\" or \"receiver\", but find \"" + c.Mode + "\"")
	}
	// ip addr
	res = res && isValidIp(&c.Addr)
	if res == false {
		return fmt.Errorf("invalid ipv4 addr \"" + c.Addr + "\"")
	}
	// Topic
	for _, topic := range c.Topics {
		res = res && isTopicNameValid(&(topic.NameIn))
		res = res && isTopicNameValid(&(topic.NameOut))
		if res == false {
			return fmt.Errorf("wrong topic name format: \"" + topic.NameIn + "\" or \"" + topic.NameOut + "\"")
		}
		res = res && (topic.Type == "sensor_msgs/msg/Image")
		if !res {
			return fmt.Errorf("wrong topic msg type, expected \"sensor_msgs/msg/Image\", but find \"" + topic.Type + "\"")
		}
	}
	return nil
}

func LoadCfg() *Config {
	args := os.Args
	if len(args) != 2 {
		fmt.Println("Usage: wrb <config_file>")
		os.Exit(0)
	}
	if _, err := os.Stat(args[1]); errors.Is(err, os.ErrNotExist) {
		slog.Info(args[1] + " not found, using default config")
		return &Config{
			Mode: "receiver",
			Addr: "localhost:8080",
			Topics: []TopicConfig{
				{
					NameIn:  "image_raw",
					NameOut: "image",
					Type:    "sensor_msgs/msg/Image",
				},
			},
		}
	}
	f, err := os.Open(args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		panic(err)
	}
	bf := make([]byte, stat.Size())
	_, err = bufio.NewReader(f).Read(bf)
	if err != nil && err != io.EOF {
		panic(err)
	}
	c := &Config{}
	err = json.Unmarshal(bf, c)
	if err != nil {
		panic(err)
	}
	err = checkCfg(c)
	if err != nil {
		panic(err)
	}

	// Print config
	slog.Info("config loaded", "config", c)
	return c
}
