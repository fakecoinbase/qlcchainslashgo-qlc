/*
 * Copyright (c) 2019 QLC Chain Team
 *
 * This software is released under the MIT License.
 * https://opensource.org/licenses/MIT
 */

package topic

import "github.com/qlcchain/go-qlc/common/types"

// MessageType a string for message type.
type MessageType byte

//  Message Type
const (
	PublishReq      MessageType = iota //PublishReq
	ConfirmReq                         //ConfirmReq
	ConfirmAck                         //ConfirmAck
	FrontierRequest                    //FrontierReq
	FrontierRsp                        //FrontierRsp
	BulkPullRequest                    //BulkPullRequest
	BulkPullRsp                        //BulkPullRsp
	BulkPushBlock                      //BulkPushBlock
	MessageResponse                    //MessageResponse
	PovStatus
	PovPublishReq
	PovBulkPullReq
	PovBulkPullRsp
)

type EventPovRecvBlockMsg struct {
	Block   *types.PovBlock
	From    types.PovBlockFrom
	MsgPeer string

	ResponseChan chan interface{}
}

type EventRPCSyncCallMsg struct {
	Name string
	In   interface{}
	Out  interface{}

	ResponseChan chan interface{}
}

type EventPublishMsg struct {
	Block *types.StateBlock
	From  string
}

type EventConfirmReqMsg struct {
	Blocks []*types.StateBlock
	From   string
}

type EventAddP2PStreamMsg struct {
	PeerID   string
	PeerInfo string
}

type EventDeleteP2PStreamMsg struct {
	PeerID string
}

type EventP2PSyncStateMsg struct {
	P2pSyncState SyncState
}

type EventBandwidthStats struct {
	TotalIn  int64
	TotalOut int64
	RateIn   float64
	RateOut  float64
}

type EventP2PConnectPeersMsg struct {
	PeersInfo []*types.PeerInfo
}

type EventP2POnlinePeersMsg struct {
	PeersInfo []*types.PeerInfo
}

type EventBroadcastMsg struct {
	Type    MessageType
	Message interface{}
}

const (
	PermissionEventNodeUpdate uint8 = iota
)

type PermissionEvent struct {
	EventType uint8
	NodeId    string
	NodeUrl   string
}

type EventPrivacySendReqMsg struct {
	RawPayload     types.HexBytes
	PrivateFrom    string
	PrivateFor     []string
	PrivateGroupID string

	ReqData interface{}
	RspChan chan *EventPrivacySendRspMsg
}

type EventPrivacySendRspMsg struct {
	ReqData interface{}

	Err        error
	EnclaveKey []byte
}

type EventPrivacyRecvReqMsg struct {
	EnclaveKey []byte

	ReqData interface{}
	RspChan chan *EventPrivacyRecvRspMsg
}

type EventPrivacyRecvRspMsg struct {
	ReqData interface{}

	Err        error
	RawPayload []byte
}
