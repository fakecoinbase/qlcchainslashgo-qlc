package test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	rpc "github.com/qlcchain/jsonrpc2"

	"github.com/qlcchain/go-qlc/chain"

	"github.com/google/uuid"

	"github.com/qlcchain/go-qlc/config"
	"github.com/qlcchain/go-qlc/ledger/process"
)

func generateChain() (func() error, *rpc.Client, *chain.LedgerService, error) {
	dir := filepath.Join(config.QlcTestDataDir(), uuid.New().String())
	cm := config.NewCfgManager(dir)
	cfgFile := cm.ConfigFile
	ls := chain.NewLedgerService(cfgFile)
	err := ls.Init()
	if err != nil {
		return nil, nil, nil, err
	}
	err = ls.Start()
	if err != nil {
		return nil, nil, nil, err
	}

	_ = json.Unmarshal([]byte(jsonTestSend), &testSendBlock)
	_ = json.Unmarshal([]byte(jsonTestReceive), &testReceiveBlock)
	_ = json.Unmarshal([]byte(jsonTestGasSend), &testSendGasBlock)
	_ = json.Unmarshal([]byte(jsonTestGasReceive), &testReceiveGasBlock)
	l := ls.Ledger
	verifier := process.NewLedgerVerifier(l)
	p, _ := verifier.Process(&testSendBlock)
	if p != process.Progress {
		return nil, nil, nil, errors.New("process send block error")
	}
	p, _ = verifier.Process(&testReceiveBlock)
	if p != process.Progress {
		return nil, nil, nil, errors.New("process receive block error")
	}
	p, _ = verifier.Process(&testSendGasBlock)
	if p != process.Progress {
		return nil, nil, nil, errors.New("process send gas block error")
	}
	p, _ = verifier.Process(&testReceiveGasBlock)
	if p != process.Progress {
		return nil, nil, nil, errors.New("process receive gas block error")
	}
	rPCService, err := chain.NewRPCService(cfgFile)
	if err != nil {
		return nil, nil, nil, err
	}
	err = rPCService.Init()
	if err != nil {
		return nil, nil, nil, err
	}
	err = rPCService.Start()
	if err != nil {
		return nil, nil, nil, err
	}
	client, err := rPCService.RPC().Attach()
	if err != nil {
		return nil, nil, nil, err
	}
	sqliteService, err := chain.NewSqliteService(cfgFile)
	if err != nil {
		return nil, nil, nil, err
	}
	err = sqliteService.Init()
	if err != nil {
		return nil, nil, nil, err
	}
	err = sqliteService.Start()
	if err != nil {
		return nil, nil, nil, err
	}
	walletService := chain.NewWalletService(cfgFile)
	err = walletService.Init()
	if err != nil {
		return nil, nil, nil, err
	}
	err = walletService.Start()
	if err != nil {
		return nil, nil, nil, err
	}
	return func() error {
		if client != nil {
			client.Close()
		}
		if err := ls.Stop(); err != nil {
			return err
		}
		if err := sqliteService.Stop(); err != nil {
			return err
		}
		if err := walletService.Stop(); err != nil {
			return err
		}
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		return nil
	}, client, ls, nil
}
