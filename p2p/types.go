package p2p

import (
	"fmt"

	"github.com/qlcchain/go-qlc/common/types"
)

// MessageType a string for message type.
type MessageType byte

//// Message interface for message.
//type Message interface {
//	MessageType() MessageType
//	MessageFrom() string
//	Data() []byte
//	Hash() types.Hash
//	Content() []byte
//}

// PeersSlice is a slice which contains peers
type PeersSlice []interface{}

//// Service net Service interface
//type Service interface {
//	common.Service
//	Node() *QlcNode
//	MessageEvent() event.EventBus
//	Broadcast(messageName string, value interface{})
//	SendMessageToPeer(messageName string, value interface{}, peerID string) error
//	//Broadcast message, except for the peerID in the parameter
//	SendMessageToPeers(messageName string, value interface{}, peerID string)
//}

// Subscriber subscriber.
type Subscriber struct {
	// msgChan chan for subscribed message.
	msgChan chan *Message

	// msgType message type to subscribe
	msgType MessageType
}

// NewSubscriber return new Subscriber instance.
func NewSubscriber(msgChan chan *Message, msgType MessageType) *Subscriber {
	return &Subscriber{msgChan, msgType}
}

// MessageType return msgTypes.
func (s *Subscriber) MessageType() MessageType {
	return s.msgType
}

// MessageChan return msgChan.
func (s *Subscriber) MessageChan() chan *Message {
	return s.msgChan
}

// Message struct
type Message struct {
	messageType MessageType
	from        string
	data        []byte //removed the header
	content     []byte //complete message data
}

// NewBaseMessage new base message
func NewMessage(messageType MessageType, from string, data []byte, content []byte) *Message {
	return &Message{messageType: messageType, from: from, data: data, content: content}
}

// MessageType get message type
func (msg *Message) MessageType() MessageType {
	return msg.messageType
}

// MessageFrom get message who send
func (msg *Message) MessageFrom() string {
	return msg.from
}

// Data get the message data
func (msg *Message) Data() []byte {
	return msg.data
}

// Content get the message content
func (msg *Message) Content() []byte {
	return msg.content
}

// Hash return the message hash
func (msg *Message) Hash() types.Hash {
	hash, _ := types.HashBytes(msg.content)
	return hash
}

// String get the message to string
func (msg *Message) String() string {
	return fmt.Sprintf("Message {type:%d; data:%s; from:%s}",
		msg.messageType,
		msg.data,
		msg.from,
	)
}
