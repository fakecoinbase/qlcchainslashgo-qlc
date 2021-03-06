// +build testnet

/*
 * Copyright (c) 2019 QLC Chain Team
 *
 * This software is released under the MIT License.
 * https://opensource.org/licenses/MIT
 */

package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	qlcchainctx "github.com/qlcchain/go-qlc/chain/context"
	"github.com/qlcchain/go-qlc/common/types"
	"github.com/qlcchain/go-qlc/common/util"
	"github.com/qlcchain/go-qlc/config"
	"github.com/qlcchain/go-qlc/crypto/random"
	"github.com/qlcchain/go-qlc/ledger"
	"github.com/qlcchain/go-qlc/ledger/process"
	"github.com/qlcchain/go-qlc/mock"
	cabi "github.com/qlcchain/go-qlc/vm/contract/abi/settlement"
)

var (
	assetParam = cabi.AssetParam{
		Owner: cabi.Contractor{
			Address: mock.Address(),
			Name:    "HKT-CSL",
		},
		Previous: mock.Hash(),
		Assets: []*cabi.Asset{
			{
				Mcc:         42,
				Mnc:         5,
				TotalAmount: 1000,
				SLAs: []*cabi.SLA{
					{
						SLAType:  cabi.SLATypeLatency,
						Priority: 0,
						Value:    30,
						Compensations: []*cabi.Compensation{
							{
								Low:  50,
								High: 60,
								Rate: 10,
							},
							{
								Low:  60,
								High: 80,
								Rate: 20.5,
							},
						},
					},
				},
			},
		},
		SignDate:  time.Now().Unix(),
		StartDate: time.Now().AddDate(0, 0, 1).Unix(),
		EndDate:   time.Now().AddDate(1, 0, 1).Unix(),
		Status:    cabi.AssetStatusActivated,
	}
)

func setupSettlementAPI(t *testing.T) (func(t *testing.T), *process.LedgerVerifier, *SettlementAPI) {
	//t.Parallel()
	dir := filepath.Join(config.QlcTestDataDir(), "api", uuid.New().String())
	_ = os.RemoveAll(dir)
	cm := config.NewCfgManager(dir)
	cm.Load()
	cc := qlcchainctx.NewChainContext(cm.ConfigFile)
	l := ledger.NewLedger(cm.ConfigFile)
	verifier := process.NewLedgerVerifier(l)
	setPovStatus(l, cc, t)
	setLedgerStatus(l, t)

	api := NewSettlement(l, cc)

	var blocks []*types.StateBlock
	if err := json.Unmarshal([]byte(mock.MockBlocks), &blocks); err != nil {
		t.Fatal(err)
	}

	for i := range blocks {
		block := blocks[i]
		if err := verifier.BlockProcess(block); err != nil {
			t.Fatal(err)
		}
	}

	return func(t *testing.T) {
		err := l.Close()
		if err != nil {
			t.Fatal(err)
		}
		err = os.RemoveAll(dir)
		if err != nil {
			t.Fatal(err)
		}
		_ = cc.Stop()
	}, verifier, api
}

func buildContactParam(addr1, addr2 types.Address, name1, name2 string) *CreateContractParam {
	return &CreateContractParam{
		PartyA: cabi.Contractor{
			Address: addr1,
			Name:    name1,
		},
		PartyB: cabi.Contractor{
			Address: addr2,
			Name:    name2,
		},
		Services: []cabi.ContractService{{
			ServiceId:   mock.Hash().String(),
			Mcc:         1,
			Mnc:         2,
			TotalAmount: 10,
			UnitPrice:   2,
			Currency:    "USD",
		}, {
			ServiceId:   mock.Hash().String(),
			Mcc:         22,
			Mnc:         1,
			TotalAmount: 30,
			UnitPrice:   4,
			Currency:    "USD",
		}},
		StartDate: time.Now().AddDate(0, 0, -1).Unix(),
		EndDate:   time.Now().AddDate(1, 0, 1).Unix(),
	}
}

func TestSettlementAPI_Integration(t *testing.T) {
	testcase, verifier, api := setupSettlementAPI(t)
	defer testcase(t)

	pccwAddr := account1.Address()
	cslAddr := account2.Address()

	param := buildContactParam(pccwAddr, cslAddr, "PCCWG", "HTK-CSL")

	if address, err := api.ToAddress(&cabi.CreateContractParam{
		PartyA:    param.PartyA,
		PartyB:    param.PartyB,
		Previous:  mock.Hash(),
		Services:  param.Services,
		SignDate:  time.Now().Unix(),
		StartDate: param.StartDate,
		EndDate:   param.EndDate,
	}); err != nil {
		t.Fatal(err)
	} else if address.IsZero() {
		t.Fatal("ToAddress failed")
	}
	account := "SAP_DIRECTS"
	customer := "SAP Mobile Services"

	if blk, err := api.GetCreateContractBlock(param); err != nil {
		t.Fatal(err)
	} else {
		//t.Log(blk)
		txHash := blk.GetHash()
		blk.Signature = account1.Sign(txHash)
		if err := verifier.BlockProcess(blk); err != nil {
			t.Fatal(err)
		}
		if rx, err := api.GetSettlementRewardsBlock(&txHash); err != nil {
			t.Fatal(err)
		} else {
			if err := verifier.BlockProcess(rx); err != nil {
				t.Fatal(err)
			}
		}

		if contracts, err := api.GetContractsAsPartyA(&pccwAddr, 10, offset(0)); err != nil {
			t.Fatal(err)
		} else if len(contracts) != 1 {
			t.Fatalf("invalid GetContractsAsPartyA len, exp: 1, act: %d", len(contracts))
		}

		if contracts, err := api.GetContractsByAddress(&pccwAddr, 10, offset(0)); err != nil {
			t.Fatal(err)
		} else if len(contracts) != 1 {
			t.Fatalf("invalid GetContractsByAddress len, exp: 1, act: %d", len(contracts))
		}

		if contracts, err := api.GetContractsAsPartyB(&cslAddr, 1, offset(0)); err != nil {
			t.Fatal(err)
		} else if len(contracts) != 1 {
			t.Fatalf("invalid contracts len, exp: 1, act: %d", len(contracts))
		} else {
			contractAddr1 := contracts[0].Address

			if blk, err := api.GetSignContractBlock(&SignContractParam{
				ContractAddress: contractAddr1,
				Address:         cslAddr,
			}); err != nil {
				t.Fatal(err)
			} else {
				blk.Signature = account2.Sign(blk.GetHash())
				if err := verifier.BlockProcess(blk); err != nil {
					t.Fatal(err)
				}
			}

			// add next stop
			if blk, err := api.GetAddNextStopBlock(&StopParam{
				StopParam: cabi.StopParam{
					ContractAddress: contractAddr1,
					StopName:        "HKTCSL",
				},
				Address: pccwAddr,
			}); err != nil {
				t.Fatal(err)
			} else {
				blk.Signature = account1.Sign(txHash)
				if err := verifier.BlockProcess(blk); err != nil {
					t.Fatal(err)
				}
			}

			if blk, err := api.GetAddNextStopBlock(&StopParam{
				StopParam: cabi.StopParam{
					ContractAddress: contractAddr1,
					StopName:        "HKTCSL-1",
				},
				Address: pccwAddr,
			}); err != nil {
				t.Fatal(err)
			} else {
				blk.Signature = account1.Sign(txHash)
				if err := verifier.BlockProcess(blk); err != nil {
					t.Fatal(err)
				}
			}

			// update next stop name
			if blk, err := api.GetUpdateNextStopBlock(&UpdateStopParam{
				UpdateStopParam: cabi.UpdateStopParam{
					ContractAddress: contractAddr1,
					StopName:        "HKTCSL-1",
					New:             "HKTCSL-2",
				},
				Address: pccwAddr,
			}); err != nil {
				t.Fatal(err)
			} else {
				blk.Signature = account1.Sign(txHash)
				if err := verifier.BlockProcess(blk); err != nil {
					t.Fatal(err)
				}
			}
			if blk, err := api.GetRemoveNextStopBlock(&StopParam{
				StopParam: cabi.StopParam{
					ContractAddress: contractAddr1,
					StopName:        "HKTCSL-2",
				},
				Address: pccwAddr,
			}); err != nil {
				t.Fatal(err)
			} else {
				blk.Signature = account1.Sign(txHash)
				if err := verifier.BlockProcess(blk); err != nil {
					t.Fatal(err)
				}
			}

			// add pre stop
			if blk, err := api.GetAddPreStopBlock(&StopParam{
				StopParam: cabi.StopParam{
					ContractAddress: contractAddr1,
					StopName:        "PCCWG",
				},
				Address: cslAddr,
			}); err != nil {
				t.Fatal(err)
			} else {
				blk.Signature = account2.Sign(txHash)
				if err := verifier.BlockProcess(blk); err != nil {
					t.Fatal(err)
				}
			}

			if blk, err := api.GetAddPreStopBlock(&StopParam{
				StopParam: cabi.StopParam{
					ContractAddress: contractAddr1,
					StopName:        "PCCWG-1",
				},
				Address: cslAddr,
			}); err != nil {
				t.Fatal(err)
			} else {
				blk.Signature = account2.Sign(txHash)
				if err := verifier.BlockProcess(blk); err != nil {
					t.Fatal(err)
				}
			}

			// update pre stop
			if blk, err := api.GetUpdatePreStopBlock(&UpdateStopParam{
				UpdateStopParam: cabi.UpdateStopParam{
					ContractAddress: contractAddr1,
					StopName:        "PCCWG-1",
					New:             "PCCWG-2",
				},
				Address: cslAddr,
			}); err != nil {
				t.Fatal(err)
			} else {
				blk.Signature = account2.Sign(txHash)
				if err := verifier.BlockProcess(blk); err != nil {
					t.Fatal(err)
				}
			}

			// remove pre stop
			if blk, err := api.GetRemovePreStopBlock(&StopParam{
				StopParam: cabi.StopParam{
					ContractAddress: contractAddr1,
					StopName:        "PCCWG-2",
				},
				Address: cslAddr,
			}); err != nil {
				t.Fatal(err)
			} else {
				blk.Signature = account2.Sign(txHash)
				if err := verifier.BlockProcess(blk); err != nil {
					t.Fatal(err)
				}
			}

			if contracts, err := api.GetContractsByStatus(&pccwAddr, "Activated", 10, offset(0)); err != nil {
				t.Fatal(err)
			} else if len(contracts) != 1 {
				t.Fatalf("invalid GetContractsByStatus len, exp: 1, act: %d", len(contracts))
			}

			if a, err := api.GetContractAddressByPartyANextStop(&pccwAddr, "HKTCSL"); err != nil {
				t.Fatal(err)
			} else if *a != contractAddr1 {
				t.Fatalf("invalid contract address, exp: %s, act: %s", contractAddr1.String(), a.String())
			}

			if a, err := api.GetContractAddressByPartyBPreStop(&cslAddr, "PCCWG"); err != nil {
				t.Fatal(err)
			} else if *a != contractAddr1 {
				t.Fatalf("invalid contract address, exp: %s, act: %s", contractAddr1.String(), a.String())
			}

			// pccw upload CDR
			cdr1 := &cabi.CDRParam{
				Index:         1111,
				SmsDt:         time.Now().Unix(),
				Account:       account,
				Customer:      customer,
				Sender:        "WeChat",
				Destination:   "85257***343",
				SendingStatus: cabi.SendingStatusSent,
				DlrStatus:     cabi.DLRStatusDelivered,
				PreStop:       "",
				NextStop:      "HKTCSL",
			}

			if blk, err := api.GetProcessCDRBlock(&pccwAddr, []*cabi.CDRParam{cdr1}); err != nil {
				t.Fatal(err)
			} else {
				blk.Signature = account1.Sign(txHash)
				if err := verifier.BlockProcess(blk); err != nil {
					t.Fatal(err)
				}
			}

			// CSL upload CDR
			cdr2 := &cabi.CDRParam{
				Index:         1111,
				SmsDt:         time.Now().Unix(),
				Sender:        "WeChat",
				Destination:   "85257***343",
				SendingStatus: cabi.SendingStatusSent,
				DlrStatus:     cabi.DLRStatusDelivered,
				PreStop:       "PCCWG",
				NextStop:      "",
			}
			if blk, err := api.GetProcessCDRBlock(&cslAddr, []*cabi.CDRParam{cdr2}); err != nil {
				t.Fatal(err)
			} else {
				blk.Signature = account2.Sign(txHash)
				if err := verifier.BlockProcess(blk); err != nil {
					t.Fatal(err)
				}
			}

			if names, err := api.GetPreStopNames(&cslAddr); err != nil {
				t.Fatal(err)
			} else if len(names) == 0 {
				t.Fatal("can not find any pre names")
			}

			if names, err := api.GetNextStopNames(&pccwAddr); err != nil {
				t.Fatal(err)
			} else if len(names) == 0 {
				t.Fatal("can not find any next names")
			}

			h, err := cdr1.ToHash()
			if err != nil {
				t.Fatal(err)
			}

			if status, err := api.GetCDRStatus(&contractAddr1, h); err != nil {
				t.Fatal(err)
			} else if status.Status != cabi.SettlementStatusSuccess {
				t.Fatalf("invalid cdr state, exp: %s, act: %s", cabi.SettlementStatusSuccess.String(), status.Status.String())
			}

			if data, err := api.GetCDRStatusByCdrData(&contractAddr1, 1111, "WeChat", "85257***343"); err != nil {
				t.Fatal(err)
			} else {
				t.Log(data)
			}

			if records, err := api.GetCDRStatusByDate(&contractAddr1, 0, 0, 10, offset(0)); err != nil {
				t.Fatal(err)
			} else if len(records) != 1 {
				t.Fatalf("invalid GetCDRStatusByDate len, exp: 1, act: %d", len(records))
			}

			if allContracts, err := api.GetAllContracts(10, offset(0)); err != nil {
				t.Fatal(err)
			} else if len(allContracts) != 1 {
				t.Fatalf("invalid GetAllContracts len, exp: 1, act: %d", len(allContracts))
			}

			if report, err := api.GetSummaryReport(&contractAddr1, 0, 0); err != nil {
				t.Fatal(err)
			} else {
				t.Log(report)
			}

			if report, err := api.GetSummaryReportByAccount(&contractAddr1, account, 0, 0); err != nil {
				t.Fatal(err)
			} else {
				if v, ok := report.Records[account]; !ok {
					t.Fatal("can not generate summary invoice")
				} else {
					t.Log(report, v)
				}
			}

			if invoice, err := api.GenerateInvoicesByAccount(&contractAddr1, account, 0, 0); err != nil {
				t.Fatal(err)
			} else {
				if len(invoice) == 0 {
					t.Fatal("invalid invoice")
				} else {
					t.Log(util.ToString(invoice))
				}
			}

			if invoices, err := api.GenerateInvoices(&pccwAddr, 0, 0); err != nil {
				t.Fatal(err)
			} else {
				t.Log(invoices)
			}
			if invoices, err := api.GenerateInvoicesByContract(&contractAddr1, 0, 0); err != nil {
				t.Fatal(err)
			} else {
				t.Log(invoices)
			}

			if report, err := api.GetSummaryReportByCustomer(&contractAddr1, customer, 0, 0); err != nil {
				t.Fatal(err)
			} else {
				if v, ok := report.Records[customer]; !ok {
					t.Fatal("can not generate summary report")
				} else {
					t.Log(report, v)
				}
			}

			if invoice, err := api.GenerateInvoicesByCustomer(&contractAddr1, customer, 0, 0); err != nil {
				t.Fatal(err)
			} else {
				if len(invoice) == 0 {
					t.Fatal("invalid invoice")
				} else {
					t.Log(util.ToString(invoice))
				}
			}
		}
	}
}

func TestSortCDRs(t *testing.T) {
	cdr1 := buildCDRStatus()
	cdr2 := buildCDRStatus()
	r := []*cabi.CDRStatus{cdr1, cdr2}
	sort.Slice(r, func(i, j int) bool {
		return sortCDRFun(r[i], r[j])
	})
	t.Log(r)
}

func TestSortCDRStatus(t *testing.T) {
	const j = `[
  {
    "contractAddress": "qlc_3n4kx38h5ou7iyupezf4prbx89yxa7zujtf8d6r8ucpf6e9x13ki1dhcdwwk",
    "params": {
      "qlc_3pbbee5imrf3aik35ay44phaugkqad5a8qkngot6by7h8pzjrwwmxwket4te": [
        {
          "index": 5285517,
          "smsDt": 1585655170,
          "sender": "Slack",
          "customer": "",
          "destination": "85257***3430",
          "sendingStatus": "Sent",
          "dlrStatus": "Delivered",
          "preStop": "MONTNETS",
          "nextStop": ""
        }
      ],
      "qlc_3pekn1xq8boq1ihpj8q96wnktxiu8cfbe5syaety3bywyd45rkyhmj8b93kq": [
        {
          "index": 5285517,
          "smsDt": 1585655170,
          "sender": "Slack",
          "customer": "",
          "destination": "85257***3430",
          "sendingStatus": "Sent",
          "dlrStatus": "Delivered",
          "preStop": "",
          "nextStop": "A2P_PCCWG"
        }
      ]
    },
    "status": "success"
  },
  {
    "contractAddress": "qlc_3mdqbk4w5utsxspss7tupwnetrs4yca68o78ybd433fnhihnftnmdw5g9pmj",
    "params": {
      "qlc_1je9h6w3o5b386oig7sb8j71sf6xr9f5ipemw8gojfcqjpk6r5hiu7z3jx3z": [
        {
          "index": 5285517,
          "smsDt": 1585655170,
          "sender": "Slack",
          "customer": "",
          "destination": "85257***3430",
          "sendingStatus": "Sent",
          "dlrStatus": "Delivered",
          "preStop": "A2P_PCCWG",
          "nextStop": ""
        }
      ],
      "qlc_3pbbee5imrf3aik35ay44phaugkqad5a8qkngot6by7h8pzjrwwmxwket4te": [
        {
          "index": 5285517,
          "smsDt": 1585655170,
          "sender": "Slack",
          "customer": "",
          "destination": "85257***3430",
          "sendingStatus": "Sent",
          "dlrStatus": "Delivered",
          "preStop": "",
          "nextStop": "CSL Hong Kong @ 3397"
        }
      ]
    },
    "status": "success"
  },
  {
    "contractAddress": "qlc_3mdqbk4w5utsxspss7tupwnetrs4yca68o78ybd433fnhihnftnmdw5g9pmj",
    "params": {
      "qlc_1je9h6w3o5b386oig7sb8j71sf6xr9f5ipemw8gojfcqjpk6r5hiu7z3jx3z": [
        {
          "index": 5285518,
          "smsDt": 1585655470,
          "sender": "WeChat",
          "customer": "",
          "destination": "85257***3431",
          "sendingStatus": "Sent",
          "dlrStatus": "Delivered",
          "preStop": "A2P_PCCWG",
          "nextStop": ""
        }
      ],
      "qlc_3pbbee5imrf3aik35ay44phaugkqad5a8qkngot6by7h8pzjrwwmxwket4te": [
        {
          "index": 5285518,
          "smsDt": 1585655470,
          "sender": "WeChat",
          "customer": "",
          "destination": "85257***3431",
          "sendingStatus": "Sent",
          "dlrStatus": "Delivered",
          "preStop": "",
          "nextStop": "CSL Hong Kong @ 3397"
        }
      ]
    },
    "status": "success"
  },
  {
    "contractAddress": "qlc_3n4kx38h5ou7iyupezf4prbx89yxa7zujtf8d6r8ucpf6e9x13ki1dhcdwwk",
    "params": {
      "qlc_3pbbee5imrf3aik35ay44phaugkqad5a8qkngot6by7h8pzjrwwmxwket4te": [
        {
          "index": 5285518,
          "smsDt": 1585655470,
          "sender": "WeChat",
          "customer": "",
          "destination": "85257***3431",
          "sendingStatus": "Sent",
          "dlrStatus": "Delivered",
          "preStop": "MONTNETS",
          "nextStop": ""
        }
      ],
      "qlc_3pekn1xq8boq1ihpj8q96wnktxiu8cfbe5syaety3bywyd45rkyhmj8b93kq": [
        {
          "index": 5285518,
          "smsDt": 1585655470,
          "sender": "WeChat",
          "customer": "",
          "destination": "85257***3431",
          "sendingStatus": "Sent",
          "dlrStatus": "Delivered",
          "preStop": "",
          "nextStop": "A2P_PCCWG"
        }
      ]
    },
    "status": "success"
  }
]
`
	var r []*CDRStatus
	if err := json.Unmarshal([]byte(j), &r); err != nil {
		t.Fatal(err)
	}
	//t.Log(util.ToIndentString(r))
	sort.Slice(r, func(i, j int) bool {
		return sortCDRStatusFun(r[i], r[j])
	})
	t.Log(util.ToIndentString(r))
}

func buildCDRStatus() *cabi.CDRStatus {
	i, _ := random.Intn(10000)
	now := time.Now().Add(time.Minute * time.Duration(i)).Unix()
	cdrParam := cabi.CDRParam{
		Index:         uint64(now / 20),
		SmsDt:         now,
		Sender:        "PCCWG",
		Destination:   "85257***343",
		SendingStatus: cabi.SendingStatusSent,
		DlrStatus:     cabi.DLRStatusDelivered,
	}
	cdr1 := cdrParam

	status := &cabi.CDRStatus{
		Params: map[string][]cabi.CDRParam{
			mock.Address().String(): {cdr1},
			mock.Address().String(): {cdr1},
		},
		Status: cabi.SettlementStatusSuccess,
	}

	return status
}

func TestSettlementAPI_GetTerminateContractBlock(t *testing.T) {
	testcase, verifier, api := setupSettlementAPI(t)
	defer testcase(t)

	pccwAddr := account1.Address()
	cslAddr := account2.Address()

	param := &CreateContractParam{
		PartyA: cabi.Contractor{
			Address: pccwAddr,
			Name:    "PCCWG",
		},
		PartyB: cabi.Contractor{
			Address: cslAddr,
			Name:    "HTK-CSL",
		},
		Services: []cabi.ContractService{{
			ServiceId:   mock.Hash().String(),
			Mcc:         1,
			Mnc:         2,
			TotalAmount: 10,
			UnitPrice:   2,
			Currency:    "USD",
		}, {
			ServiceId:   mock.Hash().String(),
			Mcc:         22,
			Mnc:         1,
			TotalAmount: 30,
			UnitPrice:   4,
			Currency:    "USD",
		}},
		StartDate: time.Now().AddDate(0, 0, -1).Unix(),
		EndDate:   time.Now().AddDate(1, 0, 1).Unix(),
	}

	if blk, err := api.GetCreateContractBlock(param); err != nil {
		t.Fatal(err)
	} else {
		//t.Log(blk)
		txHash := blk.GetHash()
		blk.Signature = account1.Sign(txHash)
		if err := verifier.BlockProcess(blk); err != nil {
			t.Fatal(err)
		}

		if contracts, err := api.GetContractsAsPartyB(&cslAddr, 1, offset(0)); err != nil {
			t.Fatal(err)
		} else if len(contracts) != 1 {
			t.Fatalf("invalid contracts len, exp: 1, act: %d", len(contracts))
		} else {
			c := contracts[0]
			contractAddr := c.Address
			if blk, err := api.GetTerminateContractBlock(&TerminateParam{
				TerminateParam: cabi.TerminateParam{
					ContractAddress: contractAddr,
					Request:         true,
				},
				Address: cslAddr,
			}); err != nil {
				t.Fatal(err)
			} else {
				blk.Signature = account2.Sign(blk.GetHash())
				if err := verifier.BlockProcess(blk); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestSettlementAPI_GetExpiredContracts(t *testing.T) {
	testcase, verifier, api := setupSettlementAPI(t)
	defer testcase(t)

	pccwAddr := account1.Address()
	cslAddr := account2.Address()

	param := &CreateContractParam{
		PartyA: cabi.Contractor{
			Address: pccwAddr,
			Name:    "PCCWG",
		},
		PartyB: cabi.Contractor{
			Address: cslAddr,
			Name:    "HTK-CSL",
		},
		Services: []cabi.ContractService{{
			ServiceId:   mock.Hash().String(),
			Mcc:         1,
			Mnc:         2,
			TotalAmount: 10,
			UnitPrice:   2,
			Currency:    "USD",
		}, {
			ServiceId:   mock.Hash().String(),
			Mcc:         22,
			Mnc:         1,
			TotalAmount: 30,
			UnitPrice:   4,
			Currency:    "USD",
		}},
		StartDate: time.Now().AddDate(0, 0, -30).Unix(),
		EndDate:   time.Now().AddDate(0, 0, -1).Unix(),
	}

	if blk, err := api.GetCreateContractBlock(param); err != nil {
		t.Fatal(err)
	} else {
		txHash := blk.GetHash()
		blk.Signature = account1.Sign(txHash)
		if err := verifier.BlockProcess(blk); err != nil {
			t.Fatal(err)
		}

		if contracts, err := api.GetContractsAsPartyB(&cslAddr, 1, offset(0)); err != nil {
			t.Fatal(err)
		} else if len(contracts) != 1 {
			t.Fatalf("invalid contracts len, exp: 1, act: %d", len(contracts))
		} else {
			c := contracts[0]
			contractAddr := c.Address
			if blk, err := api.GetSignContractBlock(&SignContractParam{
				ContractAddress: contractAddr,
				Address:         cslAddr,
			}); err != nil {
				t.Fatal(err)
			} else {
				blk.Signature = account2.Sign(blk.GetHash())
				if err := verifier.BlockProcess(blk); err != nil {
					t.Fatal(err)
				}
			}

			if contracts, err := api.GetExpiredContracts(&pccwAddr, 10, offset(0)); err != nil {
				t.Fatal(err)
			} else if len(contracts) != 1 {
				t.Fatalf("invalid GetExpiredContracts len, exp: 1, act: %d", len(contracts))
			}
		}
	}
}

type paramContainer struct {
	account1, account2 *types.Account
	name1, name2       string
	pre, next          string
}

func TestSettlementAPI_GenerateMultiPartyInvoice(t *testing.T) {
	testcase, verifier, api := setupSettlementAPI(t)
	defer testcase(t)

	params := []*paramContainer{
		{
			account1: account1,
			account2: account2,
			name1:    "MONTNETS",
			name2:    "PCCWG",
			pre:      "MONTNETS",
			next:     "A2P_PCCWG",
		}, {
			account1: account2,
			account2: account3,
			name1:    "PCCWG",
			name2:    "HKT-CSL",
			pre:      "A2P_PCCWG",
			next:     "CSL Hong Kong @ 3397",
		},
	}

	// prepare two settlement contract
	for _, p := range params {
		addr1 := p.account1.Address()
		addr2 := p.account2.Address()
		param1 := buildContactParam(addr1, addr2, p.name1, p.name2)

		if blk, err := api.GetCreateContractBlock(param1); err != nil {
			t.Fatal(err)
		} else {
			txHash := blk.GetHash()
			blk.Signature = p.account1.Sign(txHash)
			if err := verifier.BlockProcess(blk); err != nil {
				t.Fatal(err)
			}

			if contracts, err := api.GetContractsAsPartyB(&addr2, 1, offset(0)); err != nil {
				t.Fatal(err)
			} else if len(contracts) != 1 {
				t.Fatalf("invalid contracts len, exp: 1, act: %d", len(contracts))
			} else {
				contractAddr := contracts[0].Address

				if blk, err := api.GetSignContractBlock(&SignContractParam{
					ContractAddress: contractAddr,
					Address:         addr2,
				}); err != nil {
					t.Fatal(err)
				} else {
					blk.Signature = p.account2.Sign(blk.GetHash())
					if err := verifier.BlockProcess(blk); err != nil {
						t.Fatal(err)
					}
				}

				// add next stop
				if blk, err := api.GetAddNextStopBlock(&StopParam{
					StopParam: cabi.StopParam{
						ContractAddress: contractAddr,
						StopName:        p.next,
					},
					Address: addr1,
				}); err != nil {
					t.Fatal(err)
				} else {
					blk.Signature = p.account1.Sign(txHash)
					if err := verifier.BlockProcess(blk); err != nil {
						t.Fatal(err)
					}
				}

				// add pre stop
				if blk, err := api.GetAddPreStopBlock(&StopParam{
					StopParam: cabi.StopParam{
						ContractAddress: contractAddr,
						StopName:        p.pre,
					},
					Address: addr2,
				}); err != nil {
					t.Fatal(err)
				} else {
					blk.Signature = p.account2.Sign(txHash)
					if err := verifier.BlockProcess(blk); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}

	// upload CDR
	montAddr := account1.Address()
	pccwAddr := account2.Address()
	cslAddr := account3.Address()

	var contractAddr1, contractAddr2 types.Address
	if c1, err := api.GetContractsAsPartyB(&pccwAddr, 1, offset(0)); err == nil {
		contractAddr1 = c1[0].Address
	}

	if c2, err := api.GetContractsAsPartyA(&pccwAddr, 1, offset(0)); err == nil {
		contractAddr2 = c2[0].Address
	}

	cdrCount := 10
	for i := 0; i < cdrCount; i++ {
		var sender string
		if i%2 == 1 {
			sender = "WeChat"
		} else {
			sender = "Slack"
		}
		now := time.Now().Add(time.Minute * 5 * time.Duration(i))
		template := cabi.CDRParam{
			Index:         uint64(now.Unix() / 300),
			SmsDt:         now.Unix(),
			Destination:   "85257***343",
			Sender:        sender,
			SendingStatus: cabi.SendingStatusSent,
			DlrStatus:     cabi.DLRStatusDelivered,
			PreStop:       "",
			NextStop:      "",
		}
		template.Destination = template.Destination[:len(template.Destination)] + strconv.Itoa(i)

		cdr1 := template
		cdr1.NextStop = "A2P_PCCWG"

		if blk, err := api.GetProcessCDRBlock(&montAddr, []*cabi.CDRParam{&cdr1}); err != nil {
			t.Fatal(err)
		} else {
			blk.Signature = account1.Sign(blk.GetHash())
			if err := verifier.BlockProcess(blk); err != nil {
				t.Fatal(err)
			}
		}

		cdr2 := template
		cdr2.PreStop = "MONTNETS"

		if blk, err := api.GetProcessCDRBlock(&pccwAddr, []*cabi.CDRParam{&cdr2}); err != nil {
			t.Fatal(err)
		} else {
			blk.Signature = account2.Sign(blk.GetHash())
			if err := verifier.BlockProcess(blk); err != nil {
				t.Fatal(err)
			}
		}

		cdr3 := template
		cdr3.NextStop = "CSL Hong Kong @ 3397"

		if blk, err := api.GetProcessCDRBlock(&pccwAddr, []*cabi.CDRParam{&cdr3}); err != nil {
			t.Fatal(err)
		} else {
			blk.Signature = account2.Sign(blk.GetHash())
			if err := verifier.BlockProcess(blk); err != nil {
				t.Fatal(err)
			}
		}
		cdr4 := template
		cdr4.PreStop = "A2P_PCCWG"

		if blk, err := api.GetProcessCDRBlock(&cslAddr, []*cabi.CDRParam{&cdr4}); err != nil {
			t.Fatal(err)
		} else {
			blk.Signature = account3.Sign(blk.GetHash())
			if err := verifier.BlockProcess(blk); err != nil {
				t.Fatal(err)
			}
		}
	}

	if status, err := api.GetAllCDRStatus(&contractAddr1, 100, offset(0)); err == nil {
		if len(status) != cdrCount {
			t.Fatalf("invalid cdr count[%d] of %s", len(status), contractAddr1.String())
		}
	}

	if status, err := api.GetAllCDRStatus(&contractAddr2, 100, offset(0)); err == nil {
		if len(status) != cdrCount {
			t.Fatalf("invalid cdr count[%d] of %s", len(status), contractAddr2.String())
		}
	}

	if invoice, err := api.GenerateMultiPartyInvoice(&contractAddr1, &contractAddr2, 0, 0); err != nil {
		t.Fatal(err)
	} else {
		t.Log(invoice)
	}

	if report, err := api.GenerateMultiPartySummaryReport(&contractAddr1, &contractAddr2, 0, 0); err != nil {
		t.Fatal(err)
	} else {
		t.Log(report)
	}

	if cdrs, err := api.GetMultiPartyCDRStatus(&contractAddr1, &contractAddr2, 100, offset(0)); err != nil {
		t.Fatal(err)
	} else {
		t.Log(util.ToIndentString(cdrs))
	}
}

func TestAssetParam_From(t *testing.T) {
	a := &AssetParam{}
	if err := a.From(&assetParam); err != nil {
		t.Fatal(err)
	} else {
		t.Log(util.ToIndentString(a))
	}
}

func TestRegisterAssetParam_ToAssetParam(t *testing.T) {
	r := &RegisterAssetParam{
		Owner:     assetParam.Owner,
		Assets:    assetParam.Assets,
		StartDate: assetParam.StartDate,
		EndDate:   assetParam.EndDate,
		Status:    cabi.AssetStatusActivated.String(),
	}
	if param, err := r.ToAssetParam(); err != nil {
		t.Fatal(err)
	} else {
		param.Previous = mock.Hash()
		if err := param.Verify(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSettlementAPI_GetRegisterAssetBlock(t *testing.T) {
	testcase, verifier, api := setupSettlementAPI(t)
	defer testcase(t)

	r := &RegisterAssetParam{
		Owner:     assetParam.Owner,
		Assets:    assetParam.Assets,
		StartDate: assetParam.StartDate,
		EndDate:   assetParam.EndDate,
		Status:    cabi.AssetStatusActivated.String(),
	}
	address := account1.Address()
	r.Owner.Address = address

	t.Log(util.ToIndentString(r))

	if blk, err := api.GetRegisterAssetBlock(r); err != nil {
		t.Fatal(err)
	} else {
		txHash := blk.GetHash()
		blk.Signature = account1.Sign(txHash)
		if err := verifier.BlockProcess(blk); err != nil {
			t.Fatal(err)
		}

		if blk, err := api.GetSettlementRewardsBlock(&txHash); err != nil {
			t.Fatal(err)
		} else {
			txHash := blk.GetHash()
			blk.Signature = account1.Sign(txHash)
			if err := verifier.BlockProcess(blk); err != nil {
				t.Fatal(err)
			}
		}
	}

	if assets, err := api.GetAllAssets(10, offset(0)); err != nil {
		t.Fatal(err)
	} else if len(assets) != 1 {
		t.Fatalf("invalid assets len, exp: 1, act: %d", len(assets))
	}

	if assets, err := api.GetAssetsByOwner(&address, 10, offset(0)); err != nil {
		t.Fatal(err)
	} else if len(assets) != 1 {
		t.Fatalf("invalid assets len, exp: 1, act: %d", len(assets))
	} else {
		t.Log(util.ToIndentString(assets[0]))
		if asset, err := api.GetAsset(assets[0].Address); err != nil {
			t.Fatal(err)
		} else if asset == nil {
			t.Fatal("")
		} else {
			t.Log(asset)
		}
	}
}
