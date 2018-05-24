package client

import (
	"flag"
)

type Client interface {
	Recv()
	Send()
}

var (
	FromMasterCh             = make(chan *Message, 100)
	ToMasterCh               = make(chan *Message, 100)
	DisconnectedFromMasterCh = make(chan bool)

	masterHost *string
	masterPort *int
	rpc        *string
)

func init() {
	masterHost = flag.String("master-host", "127.0.0.1", "Host or IP address of locust master for distributed load testing. Defaults to 127.0.0.1.")
	masterPort = flag.Int("master-port", 5557, "The port to connect to that is used by the locust master for distributed load testing. Defaults to 5557.")
	rpc = flag.String("rpc", "zeromq", "Choose zeromq or tcp socket to communicate with master, don't mix them up.")
}
