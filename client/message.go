package client

import (
	"log"

	"github.com/ugorji/go/codec"
)

var (
	mh codec.MsgpackHandle
)

type Message struct {
	Type   string                 `codec: "type"`
	Data   map[string]interface{} `codec: "data"`
	NodeID string                 `codec: "node_id"`
}

func NewMessage(t string, data map[string]interface{}, nodeID string) (msg *Message) {
	return &Message{
		Type:   t,
		Data:   data,
		NodeID: nodeID,
	}
}

func (m *Message) serialize() (out []byte) {
	mh.StructToArray = true
	enc := codec.NewEncoderBytes(&out, &mh)
	err := enc.Encode(m)
	if err != nil {
		log.Fatal("[msgpack] encode fail")
	}
	return
}

func newMessageFromBytes(raw []byte) *Message {
	mh.StructToArray = true
	dec := codec.NewDecoderBytes(raw, &mh)
	var newMsg = &Message{}
	err := dec.Decode(newMsg)
	if err != nil {
		log.Fatal("[msgpack] decode fail")
	}
	return newMsg
}
