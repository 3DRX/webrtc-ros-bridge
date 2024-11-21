package main

import (
	"fmt"
	cfg "../config"
)
/*
	usage: 
		export GO111MODULE=off
		go run test_config.go
*/
func TestCheckfunc(c *cfg.Config) *cfg.Config {
	msg := cfg.CheckCfg(c)
	if (*msg).Status {
		return c
	} else {
		fmt.Println("[error] " + (*msg).Msg)
		return nil
	}
}

func main(){
	d := &cfg.Config{
		Mode: "receiver",
		Addr: "10.3.9.3:8080",
		Topics: []cfg.TopicConfig{
			{
				NameIn:  "image_raw",
				NameOut: "image",
				Type:    "sensor_msgs/msg/Image",
			},
			{
				NameIn:  "imsge_raw",
				NameOut: "image",
				Type:    "sensor_msgs/msg/Image",
			},
		},
	}
	res := TestCheckfunc(d)
	if res != nil {
		fmt.Println("ok config")
	}
}