package ledger

import (
	"errors"
	"fmt"
	"strings"

	"github.com/qlcchain/go-qlc/common/storage"
	"github.com/qlcchain/go-qlc/common/types"
	"github.com/qlcchain/go-qlc/common/util"
)

type UncheckedBlockStore interface {
	AddUncheckedBlock(key types.Hash, value *types.StateBlock, kind types.UncheckedKind, sync types.SynchronizedKind) error
	DeleteUncheckedBlock(key types.Hash, kind types.UncheckedKind) error
	GetUncheckedBlock(parentHash types.Hash, kind types.UncheckedKind) (*types.StateBlock, types.SynchronizedKind, error)
	GetUncheckedBlocks(visit types.UncheckedBlockWalkFunc) error
	HasUncheckedBlock(hash types.Hash, kind types.UncheckedKind) (bool, error)
	CountUncheckedBlocks() (uint64, error)

	AddGapPovBlock(height uint64, block *types.StateBlock, sync types.SynchronizedKind) error
	//GetGapPovBlock(height uint64) (types.StateBlockList, []types.SynchronizedKind, error)
	//CountGapPovBlocks() uint64
	DeleteGapPovBlock(height uint64, hash types.Hash) error
	WalkGapPovBlocksWithHeight(height uint64, visit types.GapPovBlockWalkFunc) error
	WalkGapPovBlocks(visit types.GapPovBlockWalkFunc) error

	AddGapPublishBlock(key types.Hash, blk *types.StateBlock, sync types.SynchronizedKind) error
	DeleteGapPublishBlock(key types.Hash, blkHash types.Hash) error
	GetGapPublishBlock(key types.Hash, visit types.GapPublishBlockWalkFunc) error

	AddGapDoDSettleStateBlock(key types.Hash, block *types.StateBlock, sync types.SynchronizedKind) error
	GetGapDoDSettleStateBlock(key types.Hash, visit types.GapDoDSettleStateBlockWalkFunc) error
	DeleteGapDoDSettleStateBlock(key, blkHash types.Hash) error

	PovHeightAddGap(height uint64) error
	PovHeightHasGap(height uint64) (bool, error)
	PovHeightDeleteGap(height uint64) error
}

func (l *Ledger) uncheckedKindToPrefix(kind types.UncheckedKind) storage.KeyPrefix {
	switch kind {
	case types.UncheckedKindPrevious:
		return storage.KeyPrefixUncheckedBlockPrevious
	case types.UncheckedKindLink:
		return storage.KeyPrefixUncheckedBlockLink
	case types.UncheckedKindTokenInfo:
		return storage.KeyPrefixUncheckedTokenInfo
	case types.UncheckedKindPublish:
		return storage.KeyPrefixGapPublish
	default:
		panic("bad unchecked block kind")
	}
}

func (l *Ledger) AddUncheckedBlock(key types.Hash, blk *types.StateBlock, kind types.UncheckedKind, sync types.SynchronizedKind) error {
	k, err := storage.GetKeyOfParts(l.uncheckedKindToPrefix(kind), key)
	if err != nil {
		return err
	}
	value := types.Unchecked{
		Block: blk,
		Kind:  sync,
	}
	v, err := value.Serialize()
	if err != nil {
		return err
	}
	if b, _ := l.store.Has(k); b {
		return ErrUncheckedBlockExists
	}
	return l.store.Put(k, v)
}

func (l *Ledger) GetUncheckedBlock(hash types.Hash, kind types.UncheckedKind) (*types.StateBlock, types.SynchronizedKind, error) {
	k, err := storage.GetKeyOfParts(l.uncheckedKindToPrefix(kind), hash)
	if err != nil {
		return nil, 0, err
	}

	v, err := l.store.Get(k)
	if err != nil {
		if err == storage.KeyNotFound {
			return nil, 0, ErrUncheckedBlockNotFound
		}
		return nil, 0, err
	}

	value := new(types.Unchecked)
	if err := value.Deserialize(v); err != nil {
		return nil, 0, fmt.Errorf("uncheck deserialize error: %s", err)
	}

	if value.Block.IsPrivate() {
		pl, err := l.GetBlockPrivatePayload(value.Block.GetHash())
		if err == nil {
			value.Block.SetPrivatePayload(pl)
		}
	}

	return value.Block, value.Kind, nil
}

func (l *Ledger) DeleteUncheckedBlock(key types.Hash, kind types.UncheckedKind) error {
	k, err := storage.GetKeyOfParts(l.uncheckedKindToPrefix(kind), key)
	if err != nil {
		return err
	}
	return l.store.Delete(k)
}

func (l *Ledger) HasUncheckedBlock(hash types.Hash, kind types.UncheckedKind) (bool, error) {
	k, err := storage.GetKeyOfParts(l.uncheckedKindToPrefix(kind), hash)
	if err != nil {
		return false, err
	}

	return l.store.Has(k)
}

func (l *Ledger) getUncheckedBlocks(kind types.UncheckedKind, visit types.UncheckedBlockWalkFunc) error {
	prefix, _ := storage.GetKeyOfParts(l.uncheckedKindToPrefix(kind))
	return l.store.Iterator(prefix, nil, func(key []byte, val []byte) error {
		u := new(types.Unchecked)
		if err := u.Deserialize(val); err != nil {
			return fmt.Errorf("uncheck deserialize err: %s", err)
		}

		h, err := types.BytesToHash(key[1:])
		if err != nil {
			return fmt.Errorf("uncheck kind err: %s", err)
		}

		if u.Block.IsPrivate() {
			pl, err := l.GetBlockPrivatePayload(u.Block.GetHash())
			if err == nil {
				u.Block.SetPrivatePayload(pl)
			}
		}

		if err := visit(u.Block, h, kind, u.Kind); err != nil {
			return fmt.Errorf("visit unchecked error %s", err)
		}
		return nil
	})
}

func (l *Ledger) GetUncheckedBlocks(visit types.UncheckedBlockWalkFunc) error {
	if err := l.getUncheckedBlocks(types.UncheckedKindPrevious, visit); err != nil {
		return err
	}

	if err := l.getUncheckedBlocks(types.UncheckedKindLink, visit); err != nil {
		return err
	}

	if err := l.getUncheckedBlocks(types.UncheckedKindTokenInfo, visit); err != nil {
		return err
	}

	return l.getUncheckedBlocks(types.UncheckedKindPublish, visit)
}

func (l *Ledger) CountUncheckedBlocks() (uint64, error) {
	var count uint64
	count, err := l.store.Count([]byte{byte(storage.KeyPrefixUncheckedBlockLink)})
	if err != nil {
		return 0, err
	}

	count2, err := l.store.Count([]byte{byte(storage.KeyPrefixUncheckedBlockPrevious)})
	if err != nil {
		return 0, err
	}

	count3, err := l.store.Count([]byte{byte(storage.KeyPrefixUncheckedPovHeight)})
	if err != nil {
		return 0, err
	}

	count4, err := l.store.Count([]byte{byte(storage.KeyPrefixGapPublish)})
	if err != nil {
		return 0, err
	}

	count5, err := l.store.Count([]byte{byte(storage.KeyPrefixGapDoDSettleState)})
	if err != nil {
		return 0, err
	}

	return count + count2 + count3 + count4 + count5, nil
}

func (l *Ledger) PovHeightAddGap(height uint64) error {
	k, err := storage.GetKeyOfParts(storage.KeyPrefixGapPovHeight, height)
	if err != nil {
		return err
	}

	return l.store.Put(k, nil)
}

func (l *Ledger) PovHeightHasGap(height uint64) (bool, error) {
	k, err := storage.GetKeyOfParts(storage.KeyPrefixGapPovHeight, height)
	if err != nil {
		return false, err
	}

	return l.store.Has(k)
}

func (l *Ledger) PovHeightDeleteGap(height uint64) error {
	k, err := storage.GetKeyOfParts(storage.KeyPrefixGapPovHeight, height)
	if err != nil {
		return err
	}

	return l.store.Delete(k)
}

func (l *Ledger) AddGapPovBlock(height uint64, blk *types.StateBlock, sync types.SynchronizedKind) error {
	k, err := storage.GetKeyOfParts(storage.KeyPrefixUncheckedPovHeight, height, blk.GetHash())
	if err != nil {
		return err
	}

	value := types.Unchecked{
		Block: blk,
		Kind:  sync,
	}
	v, err := value.Serialize()
	if err != nil {
		return err
	}

	return l.store.Put(k, v)
}

func (l *Ledger) DeleteGapPovBlock(height uint64, hash types.Hash) error {
	k, err := storage.GetKeyOfParts(storage.KeyPrefixUncheckedPovHeight, height, hash)
	if err != nil {
		return err
	}
	return l.store.Delete(k)
}

func (l *Ledger) WalkGapPovBlocksWithHeight(height uint64, visit types.GapPovBlockWalkFunc) error {
	var itKey []byte
	itKey = append(itKey, byte(storage.KeyPrefixUncheckedPovHeight))
	itKey = append(itKey, util.BE_Uint64ToBytes(height)...)
	return l.store.Iterator(itKey, nil, func(key []byte, val []byte) error {
		u := new(types.Unchecked)
		if err := u.Deserialize(val); err != nil {
			return fmt.Errorf("uncheck deserialize err: %s", err)
		}

		if u.Block.IsPrivate() {
			pl, err := l.GetBlockPrivatePayload(u.Block.GetHash())
			if err == nil {
				u.Block.SetPrivatePayload(pl)
			}
		}

		return visit(u.Block, height, u.Kind)
	})
}

func (l *Ledger) WalkGapPovBlocks(visit types.GapPovBlockWalkFunc) error {
	prefix, _ := storage.GetKeyOfParts(storage.KeyPrefixUncheckedPovHeight)

	return l.store.Iterator(prefix, nil, func(key []byte, val []byte) error {
		u := new(types.Unchecked)
		if err := u.Deserialize(val); err != nil {
			return fmt.Errorf("uncheck deserialize err: %s", err)
		}

		if u.Block.IsPrivate() {
			pl, err := l.GetBlockPrivatePayload(u.Block.GetHash())
			if err == nil {
				u.Block.SetPrivatePayload(pl)
			}
		}

		height := util.BE_BytesToUint64(key[1:9])
		return visit(u.Block, height, u.Kind)
	})
}

func (l *Ledger) AddGapPublishBlock(key types.Hash, blk *types.StateBlock, sync types.SynchronizedKind) error {
	k, err := storage.GetKeyOfParts(storage.KeyPrefixGapPublish, key, blk.GetHash())
	if err != nil {
		return err
	}

	if b, _ := l.store.Has(k); b {
		return ErrUncheckedBlockExists
	}
	value := types.Unchecked{
		Block: blk,
		Kind:  sync,
	}
	v, err := value.Serialize()
	if err != nil {
		return err
	}
	return l.store.Put(k, v)

}

func (l *Ledger) DeleteGapPublishBlock(key types.Hash, blkHash types.Hash) error {
	k, err := storage.GetKeyOfParts(storage.KeyPrefixGapPublish, key, blkHash)
	if err != nil {
		return err
	}

	return l.store.Delete(k)
}

func (l *Ledger) GetGapPublishBlock(key types.Hash, visit types.GapPublishBlockWalkFunc) error {
	k, err := storage.GetKeyOfParts(storage.KeyPrefixGapPublish, key)
	if err != nil {
		return err
	}

	errStr := make([]string, 0)
	err = l.store.Iterator(k, nil, func(key []byte, val []byte) error {
		u := new(types.Unchecked)
		if err := u.Deserialize(val); err != nil {
			return fmt.Errorf("uncheck deserialize err: %s", err)
		}

		if u.Block.IsPrivate() {
			pl, err := l.GetBlockPrivatePayload(u.Block.GetHash())
			if err == nil {
				u.Block.SetPrivatePayload(pl)
			}
		}

		err = visit(u.Block, u.Kind)
		if err != nil {
			errStr = append(errStr, err.Error())
			return nil
		}

		return nil
	})
	if err != nil {
		return err
	}
	if len(errStr) != 0 {
		return errors.New(strings.Join(errStr, ", "))
	}
	return nil
}

func (l *Ledger) AddGapDoDSettleStateBlock(key types.Hash, blk *types.StateBlock, sync types.SynchronizedKind) error {
	k, err := storage.GetKeyOfParts(storage.KeyPrefixGapDoDSettleState, key, blk.GetHash())
	if err != nil {
		return err
	}

	if b, _ := l.store.Has(k); b {
		return ErrUncheckedBlockExists
	}
	value := types.Unchecked{
		Block: blk,
		Kind:  sync,
	}
	v, err := value.Serialize()
	if err != nil {
		return err
	}
	return l.store.Put(k, v)
}

func (l *Ledger) GetGapDoDSettleStateBlock(key types.Hash, visit types.GapDoDSettleStateBlockWalkFunc) error {
	k, err := storage.GetKeyOfParts(storage.KeyPrefixGapDoDSettleState, key)
	if err != nil {
		return err
	}

	errStr := make([]string, 0)
	err = l.store.Iterator(k, nil, func(key []byte, val []byte) error {
		u := new(types.Unchecked)
		if err := u.Deserialize(val); err != nil {
			return fmt.Errorf("uncheck deserialize err: %s", err)
		}

		if u.Block.IsPrivate() {
			pl, err := l.GetBlockPrivatePayload(u.Block.GetHash())
			if err == nil {
				u.Block.SetPrivatePayload(pl)
			}
		}

		err = visit(u.Block, u.Kind)
		if err != nil {
			errStr = append(errStr, err.Error())
			return nil
		}

		return nil
	})
	if err != nil {
		return err
	}
	if len(errStr) != 0 {
		return errors.New(strings.Join(errStr, ", "))
	}

	return nil
}

func (l *Ledger) DeleteGapDoDSettleStateBlock(key, blkHash types.Hash) error {
	k, err := storage.GetKeyOfParts(storage.KeyPrefixGapDoDSettleState, key, blkHash)
	if err != nil {
		return err
	}

	return l.store.Delete(k)
}
