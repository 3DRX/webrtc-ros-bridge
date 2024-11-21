package config

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"net"
	"regexp"
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

type Check_msg struct {
	Status bool
	Msg string
}

func IsTopicNameValid(topic_name *string) bool {
	re := regexp.MustCompile(`^[a-z0-9_\-]+(/[a-z0-9_\-]+)*$`)
	if *topic_name == "" {
		return false
	}
	return re.MatchString(*topic_name)
}

func IsValidIp(addr *string) bool {
	// 分离主机名和端口
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// 主机名 + port
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			return false
		}
		ip = ips[0]
	}
	if ip.To4() == nil {
		return false
	}
	return true
}

func CheckCfg(c *Config ) *Check_msg{
	if c == nil {
		return &Check_msg{false, "get a null pointer!"}
	}
	res := true	
	// Mode
	res = res && (c.Mode == "sender" || c.Mode == "receiver")
	if res == false {
		return &Check_msg{false, "wrong Mode syntax, expected \"sender\" or \"receiver\", but find \"" + c.Mode + "\""}
	}
	// ip addr
	res = res && IsValidIp(&c.Addr)
	if res == false {
		return &Check_msg{false, "invalid ipv4 addr \"" + c.Addr + "\""}
	}
	// Topic
	for _, topic := range(c.Topics) {
		res = res && IsTopicNameValid(&(topic.NameIn))
		res = res && IsTopicNameValid(&(topic.NameOut))
		if res == false {
			return &Check_msg{false, "wrong topic name format: \"" + topic.NameIn + "\" or \"" + topic.NameOut + "\""}
		}
		res = res && (topic.Type == "sensor_msgs/msg/Image")
		if !res {
			return &Check_msg{false, "wrong topic msg type, expected \"sensor_msgs/msg/Image\", but find \"" + topic.Type + "\""}
		}
	}
	return &Check_msg{true, ""};
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
	// TODO: validate config
	check_msg := CheckCfg(c)
	if check_msg.Status {
		return c
	} else {
		fmt.Println("[error] " + check_msg.Msg)
		return nil
	}
}
