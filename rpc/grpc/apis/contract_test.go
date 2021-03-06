package apis

import (
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/google/uuid"
	"go.uber.org/zap"

	qctx "github.com/qlcchain/go-qlc/chain/context"
	"github.com/qlcchain/go-qlc/common/event"
	"github.com/qlcchain/go-qlc/common/topic"
	"github.com/qlcchain/go-qlc/common/types"
	"github.com/qlcchain/go-qlc/common/util"
	"github.com/qlcchain/go-qlc/common/vmcontract/contractaddress"
	"github.com/qlcchain/go-qlc/config"
	"github.com/qlcchain/go-qlc/ledger"
	"github.com/qlcchain/go-qlc/log"
	"github.com/qlcchain/go-qlc/mock"
	"github.com/qlcchain/go-qlc/rpc/api"
	pb "github.com/qlcchain/go-qlc/rpc/grpc/proto"
	pbtypes "github.com/qlcchain/go-qlc/rpc/grpc/proto/types"
)

type mockDataContractApi struct {
	l   ledger.Store
	cc  *qctx.ChainContext
	eb  event.EventBus
	sub *event.ActorSubscriber
}

func setupTestCaseContractApi(t *testing.T) (func(t *testing.T), *mockDataContractApi) {
	md := new(mockDataContractApi)

	dir := filepath.Join(config.QlcTestDataDir(), "rewards", uuid.New().String())
	_ = os.RemoveAll(dir)
	cm := config.NewCfgManager(dir)
	_, _ = cm.Load()

	cfg, _ := cm.Config()
	cfg.Privacy.Enable = true
	cfg.Privacy.PtmNode = filepath.Join(dir, "__UnitTestCase__.ipc")
	_ = cm.CommitAndSave()

	l := ledger.NewLedger(cm.ConfigFile)
	setLedgerStatus(l, t)
	povBlk, povTd := mock.GenerateGenesisPovBlock()
	l.AddPovBlock(povBlk, povTd)
	l.AddPovBestHash(povBlk.GetHeight(), povBlk.GetHash())
	l.SetPovLatestHeight(povBlk.GetHeight())
	md.l = l

	md.cc = qctx.NewChainContext(cm.ConfigFile)
	md.cc.Init(func() error {
		return nil
	})

	md.eb = md.cc.EventBus()

	md.cc.Start()

	md.sub = event.NewActorSubscriber(event.Spawn(func(ctx actor.Context) {
		switch msgReq := ctx.Message().(type) {
		case *topic.EventPrivacySendReqMsg:
			msgReq.RspChan <- &topic.EventPrivacySendRspMsg{
				EnclaveKey: util.RandomFixedBytes(32),
			}
		case *topic.EventPrivacyRecvReqMsg:
			msgReq.RspChan <- &topic.EventPrivacyRecvRspMsg{
				RawPayload: util.RandomFixedBytes(128),
			}
		}
	}), md.eb)

	err := md.sub.Subscribe(topic.EventPrivacySendReq, topic.EventPrivacyRecvReq)
	if err != nil {
		t.Fatal(err)
	}

	return func(t *testing.T) {
		_ = md.sub.UnsubscribeAll()
		_ = md.cc.Stop()
		_ = md.eb.Close()
		err := md.l.Close()
		if err != nil {
			t.Fatal(err)
		}
		err = os.RemoveAll(dir)
		if err != nil {
			t.Fatal(err)
		}
	}, md
}

func TestNewContractApi(t *testing.T) {
	tearDown, md := setupTestCaseContractApi(t)
	defer tearDown(t)

	contractApi := NewContractAPI(md.cc, md.l)
	if abi, err := contractApi.GetAbiByContractAddress(context.Background(), &pbtypes.Address{
		Address: contractaddress.BlackHoleAddress.String(),
	}); err != nil {
		t.Fatal(err)
	} else if len(abi.GetValue()) == 0 {
		t.Fatal("invalid abi")
	} else {
		if data, err := contractApi.PackContractData(context.Background(), &pb.PackContractDataRequest{
			AbiStr:     abi.GetValue(),
			MethodName: "Destroy",
			Params: []string{mock.Address().String(),
				mock.Hash().String(), mock.Hash().String(), "111", types.ZeroSignature.String()},
		}); err != nil {
			t.Fatal(err)
		} else if len(data.GetValue()) == 0 {
			t.Fatal("invalid data")
		} else {
			t.Log(hex.EncodeToString(data.GetValue()))
		}
	}

	if addressList, _ := contractApi.ContractAddressList(context.Background(), nil); len(addressList.GetAddresses()) == 0 {
		t.Fatal("can not find any on-chain contract")
	}
}

func TestContractApi_PackContractData(t *testing.T) {
	logger := log.NewLogger("TestContractApi_PackContractData")

	type fields struct {
		logger *zap.SugaredLogger
	}
	type args struct {
		abiStr     string
		methodName string
		params     []string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "address[]",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "address[]"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{`["qlc_38nm8t5rimw6h6j7wyokbs8jiygzs7baoha4pqzhfw1k79npyr1km8w6y7r8","qlc_38nm8t5rimw6h6j7wyokbs8jiygzs7baoha4pqzhfw1k79npyr1km8w6y7r8"]`},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "hash[]",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "hash[]"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{`["2C353DA641277FD8379354307A54BECE090C51E52FB460EA5A8674B702BDCE5E","2C353DA641277FD8379354307A54BECE090C51E52FB460EA5A8674B702BDCE5E"]`},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "tokenId[]",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "tokenId[]"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{`["2C353DA641277FD8379354307A54BECE090C51E52FB460EA5A8674B702BDCE5E","2C353DA641277FD8379354307A54BECE090C51E52FB460EA5A8674B702BDCE5E"]`},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "string[]",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "string[]"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{`["2C353DA641277FD8379354307A54BECE090C51E52FB460EA5A8674B702BDCE5E","2C353DA641277FD8379354307A54BECE090C51E52FB460EA5A8674B702BDCE5E"]`},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "signature[]",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "signature[]"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{`["bb5c2e6d3b30f2edd749669452a447c0dfd45538edc09d32e407c22ba4c728f7945aec3dd253405b2b7eb81543e081a91edccf10362a8bbe722b75021305d901","bb5c2e6d3b30f2edd749669452a447c0dfd45538edc09d32e407c22ba4c728f7945aec3dd253405b2b7eb81543e081a91edccf10362a8bbe722b75021305d901"]`},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "int8",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "int8"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{"100"},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "int16",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "int16"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{"100"},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "int32",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "int32"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{"100"},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "int64",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "int64"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{"100"},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "int128",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "int128"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{"1011111110"},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "uint8",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "uint8"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{"100"},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "uint16",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "uint16"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{"100"},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "uint32",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "uint32"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{"100"},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "uint64",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "uint64"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{"100"},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "uint128",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "uint128"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{"1011111110"},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "bytes32",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "bytes32"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{"2C353DA641277FD8379354307A54BECE090C51E52FB460EA5A8674B702BDCE5E"},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "bytes",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "bytes"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{"2C353DA641277FD8379354307A54BECE090C51E52FB460EA5A8674B702BDCE5E"},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "bool[]",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "bool[]"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{`[true, false]`},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "bool",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "bool"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{`false`},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "int8[]",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "int8[]"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{`[11, 22]`},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "int16[]",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "int16[]"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{`[111, 123]`},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "int32[]",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "int32[]"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{`[111, 123]`},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "int64[]",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "int64[]"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{`[111, 123]`},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "uint8[]",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "uint8[]"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{`[11, 22]`},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "uint16[]",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "uint16[]"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{`[111, 123]`},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "uint32[]",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "uint32[]"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{`[111, 123]`},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "uint64[]",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "uint64[]"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{`[111, 123]`},
			},
			want:    nil,
			wantErr: false,
		}, {
			name: "uint64_error",
			fields: fields{
				logger: logger,
			},
			args: args{
				abiStr: `[
  {
    "type": "function",
    "name": "Destroy",
    "inputs": [
      {
        "name": "value",
        "type": "uint64"
      }
    ]
  }
]
`,
				methodName: "Destroy",
				params:     []string{`[111, 123]`},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ContractAPI{
				logger: tt.fields.logger,
			}
			_, err := c.PackContractData(context.Background(), &pb.PackContractDataRequest{
				AbiStr:     tt.args.abiStr,
				MethodName: tt.args.methodName,
				Params:     tt.args.params,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("PackContractData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			//if !reflect.DeepEqual(got, tt.want) {
			//	t.Errorf("PackContractData() got = %v, want %v", got, tt.want)
			//}
		})
	}
}

func TestContractAPI_PackContractData(t *testing.T) {
	tearDown, md := setupTestCaseContractApi(t)
	defer tearDown(t)
	contractApi := NewContractAPI(md.cc, md.l)
	paraList := []string{
		hex.EncodeToString(util.RandomFixedBytes(32)),
		hex.EncodeToString(util.RandomFixedBytes(32)),
	}
	data, err := contractApi.PackChainContractData(context.Background(), &pb.PackChainContractDataRequest{
		ContractAddress: toAddressValue(contractaddress.PrivacyDemoKVAddress),
		MethodName:      "PrivacyDemoKVSet",
		Params:          paraList,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(data)

	sendPara := &api.ContractSendBlockPara{
		Address:   config.GenesisAddress(),
		To:        contractaddress.PrivacyDemoKVAddress,
		TokenName: "QLC",
		Amount:    types.NewBalance(0),
		Data:      data.GetValue(),

		PrivateFrom: util.RandomFixedString(32),
		PrivateFor:  []string{util.RandomFixedString(32)},
	}
	_, err = contractApi.GenerateSendBlock(context.Background(), &pb.ContractSendBlockPara{
		Address:        toAddressValue(sendPara.Address),
		TokenName:      sendPara.TokenName,
		To:             toAddressValue(sendPara.To),
		Amount:         toBalanceValue(sendPara.Amount),
		Data:           sendPara.Data,
		PrivateFrom:    sendPara.PrivateFrom,
		PrivateFor:     sendPara.PrivateFor,
		PrivateGroupID: sendPara.PrivateGroupID,
		EnclaveKey:     sendPara.EnclaveKey,
	})
	if err == nil {
		t.Fatal(err)
	}

	_, err = contractApi.GenerateRewardBlock(context.Background(), &pb.ContractRewardBlockPara{
		SendHash: toHashValue(mock.Hash()),
	})
	if err == nil {
		t.Fatal(err)
	}
}
