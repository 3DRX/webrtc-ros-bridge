# webrtc-ros-bridge

![Tests](https://github.com/3DRX/webrtc-ros-bridge/actions/workflows/test.yml/badge.svg)

## Prerequisites

- ROS2 humble, merge installed
(tested on debian 12 with ROS2 humble build from source
with command `colcon build --merge-install`)
- libvpx-dev (deb package)
- `go mod tidy` to get all go deps
    - Note that it's expected to see errors of not finding package `github.com/3DRX/webrtc-ros-bridge/rclgo_gen`,
    since it's part of the codegen using `github.com/tiiuae/rclgo`, you can just ignore it.

## Build

Source your ros2 workspace setup.sh, and then
```
make
```

## Dev

For editor use, it's better to `source ./cgo-flags.env`
before opening editor for language server to work.

## Run

`wrb` cli can be configured to be either the sender or the receiver,
the config is load from a json file specified by `wrb <path_to.json>`.

> [!IMPORTANT]  
> Don't forget to remove comments in json.

### Sender

```json
{
    "mode": "sender",
    "addr": "localhost:8080",
    "topics": [
        {
            "name_in": "image_raw", // input image topic name
            "name_out": "image_out", // doesn't really matter
            "type": "sensor_msgs/msg/Image"
        }
    ]
}
```

### Receiver

```json
{
    "mode": "receiver",
    "addr": "localhost:8080",
    "topics": [
        {
            "name_in": "image_raw", // doesn't really matter
            "name_out": "image", // output image topic name
            "type": "sensor_msgs/msg/Image"
        }
    ]
}
```

## Acknowledgment

- [pion](https://github.com/pion). It's awesome.
- [webrtc_ros](https://github.com/RobotWebTools/webrtc_ros).
Infact, the receiver of this project is compatable with webrtc_ros server node.
- [ros2](https://github.com/ros2)
