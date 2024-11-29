include ./cgo-flags.env

# Remove quotes from CGO_CFLAGS and CGO_LDFLAGS
CFLAGS := $(shell echo $(CGO_CFLAGS) | sed "s/^'//;s/'$$//")
LDFLAGS := $(shell echo $(CGO_LDFLAGS) | sed "s/^'//;s/'$$//")

webrtc-ros-bridge-client: receiver/peer_connection_channel/libvp8decoder.so rclgo_gen cgo-flags.env
	CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go build -o wrb

receiver/peer_connection_channel/libvp8decoder.so: receiver/peer_connection_channel/vp8_decoder.c receiver/peer_connection_channel/vp8_decoder.h
	cd receiver/peer_connection_channel && gcc -shared -o libvp8decoder.so -fPIC vp8_decoder.c $(pkg-config --cflags --libs vpx) $(CFLAGS) $(LDFLAGS)

receiver/ros_channel/msgs cgo-flags.env:
	go run github.com/tiiuae/rclgo/cmd/rclgo-gen generate -d rclgo_gen

test: rclgo_gen cgo-flags.env
	CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test `go list ./... | grep -v "/rclgo_gen"`

clean:
	rm -rf wrb peer_connection_channel/libvp8decoder.so ros_channel/msgs cgo-flags.env rclgo_gen

.PHONY: test clean
