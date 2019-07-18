/*
 * Copyright (c) 2019 QLC Chain Team
 *
 * This software is released under the MIT License.
 * https://opensource.org/licenses/MIT
 */

package vmstore

import (
	"bytes"
	"errors"

	"github.com/dgraph-io/badger"
	"github.com/qlcchain/go-qlc/common/types"
	"github.com/qlcchain/go-qlc/ledger"
	"github.com/qlcchain/go-qlc/ledger/db"
	"github.com/qlcchain/go-qlc/log"
	"github.com/qlcchain/go-qlc/trie"
	"go.uber.org/zap"
)

type ContractStore interface {
	GetStorage(prefix, key []byte) ([]byte, error)
	SetStorage(prefix, key []byte, value []byte) error
	Iterator(prefix []byte, fn func(key []byte, value []byte) error) error

	CalculateAmount(block *types.StateBlock) (types.Balance, error)
	IsUserAccount(address types.Address) (bool, error)
	GetAccountMeta(address types.Address) (*types.AccountMeta, error)
	GetTokenMeta(address types.Address, tokenType types.Hash) (*types.TokenMeta, error)
	GetStateBlock(hash types.Hash) (*types.StateBlock, error)
	HasTokenMeta(address types.Address, token types.Hash) (bool, error)
	SaveStorage(txns ...db.StoreTxn) error
}

const (
	idPrefixStorage = 100
)

var (
	ErrStorageExists   = errors.New("storage already exists")
	ErrStorageNotFound = errors.New("storage not found")
)

type VMContext struct {
	ledger *ledger.Ledger
	logger *zap.SugaredLogger
	Cache  *VMCache
}

func NewVMContext(l *ledger.Ledger) *VMContext {
	//TODO: fix trie
	t := trie.NewTrie(l.Store, nil, trie.NewSimpleTrieNodePool())
	return &VMContext{
		ledger: l,
		logger: log.NewLogger("vm_context"),
		Cache:  NewVMCache(t),
	}
}

func (v *VMContext) GetLogger() *zap.SugaredLogger {
	return v.logger
}

func (v *VMContext) IsUserAccount(address types.Address) (bool, error) {
	if _, err := v.ledger.HasAccountMeta(address); err == nil {
		return true, nil
	} else {
		return false, err
	}
}

func getStorageKey(prefix, key []byte) []byte {
	var storageKey []byte
	storageKey = append(storageKey, []byte{idPrefixStorage}...)
	storageKey = append(storageKey, prefix...)
	storageKey = append(storageKey, key...)
	return storageKey
}

func (v *VMContext) GetStorage(prefix, key []byte) ([]byte, error) {
	storageKey := getStorageKey(prefix, key)
	if storage := v.Cache.GetStorage(storageKey); storage == nil {
		if val, err := v.get(storageKey); err == nil {
			return val, nil
		} else {
			return nil, err
		}
	} else {
		return storage, nil
	}
}

func (v *VMContext) SetStorage(prefix, key []byte, value []byte) error {
	storageKey := getStorageKey(prefix, key)

	v.Cache.SetStorage(storageKey, value)

	return nil
}

func (v *VMContext) Iterator(prefix []byte, fn func(key []byte, value []byte) error) error {
	txn := v.ledger.Store.NewTransaction(false)
	defer func() {
		txn.Discard()
	}()

	err := txn.Iterator(idPrefixStorage, func(key []byte, val []byte, b byte) error {
		if bytes.HasPrefix(key[1:], prefix) {
			err := fn(key, val)
			if err != nil {
				v.logger.Error(err)
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (v *VMContext) CalculateAmount(block *types.StateBlock) (types.Balance, error) {
	b, err := v.ledger.CalculateAmount(block)
	if err != nil {
		v.logger.Error("calculate amount error: ", err)
		return types.ZeroBalance, err
	}
	return b, nil
}

func (v *VMContext) GetAccountMeta(address types.Address) (*types.AccountMeta, error) {
	return v.ledger.GetAccountMeta(address)
}

func (v *VMContext) SaveStorage(txns ...db.StoreTxn) error {
	storage := v.Cache.storage
	for k, val := range storage {
		err := v.set([]byte(k), val, txns...)
		if err != nil {
			v.logger.Error(err)
			return err
		}
	}
	return nil
}

func (v *VMContext) SaveTrie(txns ...db.StoreTxn) error {
	fn, err := v.Cache.Trie().Save(txns...)
	if err != nil {
		return err
	}
	fn()
	return nil
}

func (v *VMContext) get(key []byte) ([]byte, error) {
	txn := v.ledger.Store.NewTransaction(false)
	defer func() {
		txn.Commit(nil)
		txn.Discard()
	}()

	var storage []byte
	err := txn.Get(key, func(val []byte, b byte) (err error) {
		storage = val
		return nil
	})
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, ErrStorageNotFound
		}
		return nil, err
	}
	return storage, nil
}

func (v *VMContext) set(key []byte, value []byte, txns ...db.StoreTxn) error {
	var txn db.StoreTxn
	if len(txns) > 0 {
		txn = txns[0]
	} else {
		txn = v.ledger.Store.NewTransaction(true)
		defer func() {
			txn.Commit(nil)
			txn.Discard()
		}()
	}

	//err := txn.Get(key, func(bytes []byte, b byte) error {
	//	return nil
	//})
	//if err == nil {
	//	return ErrStorageExists
	//} else if err != badger.ErrKeyNotFound {
	//	return err
	//}
	return txn.Set(key, value)
}

func (v *VMContext) HasTokenMeta(address types.Address, token types.Hash) (bool, error) {
	return v.ledger.HasTokenMeta(address, token)
}

func (v *VMContext) GetTokenMeta(address types.Address, token types.Hash) (*types.TokenMeta, error) {
	return v.ledger.GetTokenMeta(address, token)
}

func (v *VMContext) GetRepresentation(address types.Address) (*types.Benefit, error) {
	return v.ledger.GetRepresentation(address)
}

func (v *VMContext) GetStateBlock(hash types.Hash) (*types.StateBlock, error) {
	return v.ledger.GetStateBlock(hash)
}

func (v *VMContext) GetPovHeaderByHeight(height uint64) (*types.PovHeader, error) {
	return v.ledger.GetPovHeaderByHeight(height)
}

func (v *VMContext) GetPovBlockByHeight(height uint64) (*types.PovBlock, error) {
	return v.ledger.GetPovBlockByHeight(height)
}

func (v *VMContext) GetLatestPovBlock() (*types.PovBlock, error) {
	return v.ledger.GetLatestPovBlock()
}
