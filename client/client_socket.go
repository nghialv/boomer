package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

type socketClient struct {
	conn *net.TCPConn
}

func newSocketClient(masterHost string, masterPort int) *socketClient {
	serverAddr := fmt.Sprintf("%s:%d", masterHost, masterPort)
	tcpAddr, _ := net.ResolveTCPAddr("tcp", serverAddr)
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		log.Fatalf("Failed to connect to the Locust master: %s %s", serverAddr, err)
	}
	conn.SetNoDelay(true)
	newClient := &socketClient{
		conn: conn,
	}
	go newClient.Recv()
	go newClient.Send()
	return newClient
}

func (c *socketClient) recvBytes(length int) []byte {
	buf := make([]byte, length)
	for length > 0 {
		n, err := c.conn.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		length = length - n
	}
	return buf
}

func (c *socketClient) Recv() {
	for {
		h := c.recvBytes(4)
		msgLength := binary.BigEndian.Uint32(h)
		msg := c.recvBytes(int(msgLength))
		msgFromMasterCh := newMessageFromBytes(msg)
		FromMasterCh <- msgFromMasterCh
	}

}

func (c *socketClient) Send() {
	for {
		select {
		case msg := <-ToMasterCh:
			c.sendMessage(msg)
			if msg.Type == "quit" {
				DisconnectedFromMasterCh <- true
			}
		}
	}
}

func (c *socketClient) sendMessage(msg *Message) {
	packed := msg.serialize()
	buf := new(bytes.Buffer)

	// use a fixed length header that indicates the length of the body
	// -----------------
	// | length | body |
	// -----------------

	binary.Write(buf, binary.BigEndian, int32(len(packed)))
	buf.Write(packed)
	c.conn.Write(buf.Bytes())
}
