/*
 * Copyright (c) 2019 QLC Chain Team
 *
 * This software is released under the MIT License.
 * https://opensource.org/licenses/MIT
 */

package process

import (
	"github.com/qlcchain/go-qlc/common/types"
)

type ProcessResult byte

const (
	Progress ProcessResult = iota
	BadWork
	BadSignature
	BadHash
	BadMerkleRoot
	BadTarget
	Old
	Fork
	GapPrevious
	GapSource
	GapSmartContract
	GapTransaction
	BalanceMismatch
	UnReceivable
	InvalidData
	InvalidTime
	InvalidTxNum
	Other
)

func (r ProcessResult) String() string {
	switch r {
	case Progress:
		return "Progress"
	case BadWork:
		return "BadWork"
	case BadSignature:
		return "BadSignature"
	case BadHash:
		return "BadHash"
	case BadMerkleRoot:
		return "BadMerkleRoot"
	case BadTarget:
		return "BadTarget"
	case Old:
		return "Old"
	case Fork:
		return "Fork"
	case GapPrevious:
		return "GapPrevious"
	case GapSource:
		return "GapSource"
	case GapSmartContract:
		return "GapSmartContract"
	case GapTransaction:
		return "GapTransaction"
	case BalanceMismatch:
		return "BalanceMismatch"
	case UnReceivable:
		return "UnReceivable"
	case InvalidData:
		return "InvalidData"
	case InvalidTxNum:
		return "InvalidTxNum"
	default:
		return "<invalid>"
	}
}

type BlockVerifier interface {
	//BlockCheck check block valid
	BlockCheck(block types.Block) (ProcessResult, error)
	//Process check block and process block to badger
	Process(block types.Block) (ProcessResult, error)
}
