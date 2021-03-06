package api

import (
	"math/big"
	"testing"

	chainctx "github.com/qlcchain/go-qlc/chain/context"
	"github.com/qlcchain/go-qlc/common"
	"github.com/qlcchain/go-qlc/common/types"
	"github.com/qlcchain/go-qlc/common/vmcontract/contractaddress"
	"github.com/qlcchain/go-qlc/config"
	"github.com/qlcchain/go-qlc/mock"
	cabi "github.com/qlcchain/go-qlc/vm/contract/abi"
	"github.com/qlcchain/go-qlc/vm/vmstore"
)

func TestMinerApi_GetAvailRewardInfo(t *testing.T) {
	clear, l, cfgFile := getTestLedger()
	if l == nil {
		t.Fatal()
	}
	defer clear()

	cc := chainctx.NewChainContext(cfgFile)
	cfg, _ := cc.Config()
	m := NewMinerApi(cfg, l)
	account := mock.Address()

	ri, _ := m.GetAvailRewardInfo(account)
	if ri != nil {
		t.Fatal()
	}

	pb := addPovBlock(l, nil, 100)
	ri, _ = m.GetAvailRewardInfo(account)
	if ri == nil || ri.NeedCallReward {
		t.Fatal()
	}

	pb = addPovBlock(l, pb, common.PovMinerRewardHeightStart+uint64(common.POVChainBlocksPerDay)+common.PovMinerRewardHeightGapToLatest)
	ri, _ = m.GetAvailRewardInfo(account)
	if ri == nil || ri.NeedCallReward {
		t.Fatal()
	}

	ds := types.NewPovMinerDayStat()
	it := types.NewPovMinerStatItem()
	it.FirstHeight = 1440
	it.LastHeight = 2880
	it.BlockNum = 120
	it.RewardAmount = types.NewBalance(10)
	ds.DayIndex = uint32(common.PovMinerRewardHeightStart / uint64(common.POVChainBlocksPerDay))
	ds.MinerStats[account.String()] = it
	l.AddPovMinerStat(ds)
	ri, _ = m.GetAvailRewardInfo(account)
	if ri == nil || !ri.NeedCallReward {
		t.Fatal()
	}
}

func TestMinerApi_UnpackRewardData(t *testing.T) {
	clear, l, cfgFile := getTestLedger()
	if l == nil {
		t.Fatal()
	}
	defer clear()

	cc := chainctx.NewChainContext(cfgFile)
	cfg, _ := cc.Config()
	m := NewMinerApi(cfg, l)
	param := new(RewardParam)
	param.Coinbase = mock.Address()
	param.Beneficial = mock.Address()
	param.RewardBlocks = 10
	param.StartHeight = 0
	param.EndHeight = 1439
	param.RewardAmount = big.NewInt(100)
	data, _ := m.GetRewardData(param)

	r, err := m.UnpackRewardData(data)
	if err != nil || r.Coinbase != param.Coinbase || r.Beneficial != param.Beneficial || r.EndHeight != param.EndHeight ||
		r.StartHeight != param.StartHeight || r.RewardBlocks != param.RewardBlocks || r.RewardAmount.Cmp(param.RewardAmount) != 0 {
		t.Fatal()
	}
}

func TestMinerApi_GetRewardSendBlock(t *testing.T) {
	clear, l, cfgFile := getTestLedger()
	if l == nil {
		t.Fatal()
	}
	defer clear()

	cc := chainctx.NewChainContext(cfgFile)
	cfg, _ := cc.Config()
	m := NewMinerApi(cfg, l)
	param := new(RewardParam)
	param.RewardBlocks = 10
	param.StartHeight = common.PovMinerRewardHeightStart
	param.EndHeight = common.PovMinerRewardHeightStart + uint64(common.POVChainBlocksPerDay) - 1
	param.RewardAmount = big.NewInt(100)

	blk, _ := m.GetRewardSendBlock(param)
	if blk != nil {
		t.Fatal()
	}

	param.Coinbase = mock.Address()
	blk, _ = m.GetRewardSendBlock(param)
	if blk != nil {
		t.Fatal()
	}

	param.Beneficial = mock.Address()
	blk, _ = m.GetRewardSendBlock(param)
	if blk != nil {
		t.Fatal()
	}

	am := mock.AccountMeta(param.Coinbase)
	l.AddAccountMeta(am, l.Cache().GetCache())
	blk, _ = m.GetRewardSendBlock(param)
	if blk != nil {
		t.Fatal()
	}

	am.Tokens[0].Type = config.ChainToken()
	l.UpdateAccountMeta(am, l.Cache().GetCache())
	blk, _ = m.GetRewardSendBlock(param)
	if blk != nil {
		t.Fatal()
	}

	pb, td := mock.GeneratePovBlock(nil, 0)
	pb.Header.BasHdr.Height = uint64(common.POVChainBlocksPerDay) * 4
	l.AddPovBlock(pb, td)
	l.SetPovLatestHeight(pb.Header.BasHdr.Height)
	l.AddPovBestHash(pb.Header.BasHdr.Height, pb.GetHash())

	ds := types.NewPovMinerDayStat()
	it := types.NewPovMinerStatItem()
	it.FirstHeight = param.StartHeight
	it.LastHeight = param.EndHeight
	it.BlockNum = uint32(param.RewardBlocks)
	it.RewardAmount = types.Balance{Int: param.RewardAmount}
	ds.DayIndex = uint32(common.PovMinerRewardHeightStart / uint64(common.POVChainBlocksPerDay))
	ds.MinerStats[param.Coinbase.String()] = it
	l.AddPovMinerStat(ds)

	blk, err := m.GetRewardSendBlock(param)
	if blk == nil {
		t.Fatal(err)
	}
}

func TestMinerApi_GetRewardRecvBlock(t *testing.T) {
	clear, l, cfgFile := getTestLedger()
	if l == nil {
		t.Fatal()
	}
	defer clear()

	cc := chainctx.NewChainContext(cfgFile)
	cfg, _ := cc.Config()
	m := NewMinerApi(cfg, l)

	sendBlock := mock.StateBlockWithoutWork()
	param := new(RewardParam)
	param.Coinbase = mock.Address()
	param.Beneficial = mock.Address()
	param.RewardBlocks = 10
	param.StartHeight = 0
	param.EndHeight = 1439
	param.RewardAmount = big.NewInt(100)
	sendBlock.Data, _ = m.GetRewardData(param)

	blk, _ := m.GetRewardRecvBlock(sendBlock)
	if blk != nil {
		t.Fatal()
	}

	sendBlock.Type = types.ContractSend
	blk, _ = m.GetRewardRecvBlock(sendBlock)
	if blk != nil {
		t.Fatal()
	}

	sendBlock.Link = contractaddress.MinerAddress.ToHash()
	blk, _ = m.GetRewardRecvBlock(sendBlock)
	if blk != nil {
		t.Fatal()
	}

	param.StartHeight = common.PovMinerRewardHeightStart
	param.EndHeight = param.StartHeight + uint64(common.POVChainBlocksPerDay)
	sendBlock.Data, _ = m.GetRewardData(param)
	sendBlock.Address = param.Coinbase
	blk, err := m.GetRewardRecvBlock(sendBlock)
	if blk == nil {
		t.Fatal(err)
	}
}

func TestMinerApi_GetRewardRecvBlockBySendHash(t *testing.T) {
	clear, l, cfgFile := getTestLedger()
	if l == nil {
		t.Fatal()
	}
	defer clear()

	cc := chainctx.NewChainContext(cfgFile)
	cfg, _ := cc.Config()
	m := NewMinerApi(cfg, l)

	sendBlock := mock.StateBlockWithoutWork()
	param := new(RewardParam)
	param.Coinbase = mock.Address()
	param.Beneficial = mock.Address()
	param.RewardBlocks = 10
	param.StartHeight = common.PovMinerRewardHeightStart
	param.EndHeight = param.StartHeight + uint64(common.POVChainBlocksPerDay)
	param.RewardAmount = big.NewInt(100)
	sendBlock.Data, _ = m.GetRewardData(param)
	sendBlock.Address = param.Coinbase
	sendBlock.Type = types.ContractSend
	sendBlock.Link = contractaddress.MinerAddress.ToHash()
	l.AddStateBlock(sendBlock)

	blk, _ := m.GetRewardRecvBlockBySendHash(sendBlock.GetHash())
	if blk == nil {
		t.Fatal()
	}
}

func TestMinerApi_GetRewardHistory(t *testing.T) {
	clear, l, cfgFile := getTestLedger()
	if l == nil {
		t.Fatal()
	}
	defer clear()

	cc := chainctx.NewChainContext(cfgFile)
	cfg, _ := cc.Config()
	m := NewMinerApi(cfg, l)
	ctx := vmstore.NewVMContext(l, &contractaddress.MinerAddress)
	account := mock.Address()

	timeStamp := common.TimeNow().Unix()
	data, _ := cabi.MinerABI.PackVariable(cabi.VariableNameMinerReward, uint64(1440), uint64(100), timeStamp, big.NewInt(200))
	err := ctx.SetStorage(contractaddress.MinerAddress.Bytes(), account[:], data)
	if err != nil {
		t.Fatal(err)
	}

	err = l.SaveStorage(vmstore.ToCache(ctx))
	if err != nil {
		t.Fatal(err)
	}

	h, _ := m.GetRewardHistory(account)
	if h == nil || h.RewardBlocks != 100 || h.RewardAmount.Compare(types.NewBalance(200)) != types.BalanceCompEqual ||
		h.LastEndHeight != 1440 || h.LastRewardTime != timeStamp {
		t.Fatal()
	}
}
