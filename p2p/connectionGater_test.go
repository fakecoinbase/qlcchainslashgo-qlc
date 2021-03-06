package p2p

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/qlcchain/go-qlc/chain/context"
	"github.com/qlcchain/go-qlc/common/topic"
	"github.com/qlcchain/go-qlc/config"
	"github.com/qlcchain/go-qlc/ledger"
	"github.com/qlcchain/go-qlc/mock"
)

func TestWhiteListMode(t *testing.T) {
	removeDir := filepath.Join(config.QlcTestDataDir(), "whiteListMode")
	//bootNode config
	dir := filepath.Join(config.QlcTestDataDir(), "whiteListMode", uuid.New().String(), config.QlcConfigFile)
	cc := context.NewChainContext(dir)
	cfg, _ := cc.Config()
	cfg.P2P.Listen = "/ip4/127.0.0.1/tcp/18000"
	cfg.P2P.Discovery.MDNSEnabled = false
	cfg.LogLevel = "warn"
	cfg.P2P.IsBootNode = true
	cfg.WhiteList.Enable = true
	w1 := &config.WhiteListInfo{
		PeerId: "Qmc5VuY4Bys47oyJM1tPoN784ViNDYrh2vEQwGqZBMK5RX",
		Addr:   "127.0.0.1:18002",
	}
	cfg.WhiteList.WhiteListInfos = append(cfg.WhiteList.WhiteListInfos, w1)
	http.HandleFunc("/wlm/bootNode", func(w http.ResponseWriter, r *http.Request) {
		bootNode := cfg.P2P.Listen + "/p2p/" + cfg.P2P.ID.PeerID
		_, _ = fmt.Fprintf(w, bootNode)
	})
	go func() {
		if err := http.ListenAndServe("127.0.0.1:18001", nil); err != nil {
			t.Fatal(err)
		}
	}()

	//start bootNode
	setPovStatus(cc, t)
	node, err := NewQlcService(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = node.Start()
	if err != nil {
		t.Fatal(err)
	}
	//node1 config
	dir1 := filepath.Join(config.QlcTestDataDir(), "whiteListMode", uuid.New().String(), config.QlcConfigFile)
	cc1 := context.NewChainContext(dir1)
	cfg1, _ := cc1.Config()
	cfg1.P2P.Listen = "/ip4/127.0.0.1/tcp/18002"
	cfg1.P2P.BootNodes = []string{"http://127.0.0.1:18001/wlm/bootNode"}
	cfg1.P2P.Discovery.MDNSEnabled = false
	cfg1.P2P.Discovery.DiscoveryInterval = 1
	cfg1.LogLevel = "warn"
	cfg1.WhiteList.Enable = true

	//start bootNode
	setPovStatus(cc1, t)
	//start1 node
	node1, err := NewQlcService(dir1)
	if err != nil {
		t.Fatal(err)
	}
	err = node1.Start()
	if err != nil {
		t.Fatal(err)
	}
	pm := &topic.PermissionEvent{
		NodeId:  node1.node.ID.Pretty(),
		NodeUrl: "127.0.0.1:18002",
	}
	node.msgEvent.Publish(topic.EventPermissionNodeUpdate, pm)
	//remove test file
	defer func() {
		err = node.Stop()
		if err != nil {
			t.Fatal(err)
		}
		err = node1.Stop()
		if err != nil {
			t.Fatal(err)
		}
		err = node.msgService.ledger.Close()
		if err != nil {
			t.Fatal(err)
		}
		err = node1.msgService.ledger.Close()
		if err != nil {
			t.Fatal(err)
		}
		err = os.RemoveAll(removeDir)
		if err != nil {
			t.Fatal(err)
		}
	}()

	ticker1 := time.NewTicker(60 * time.Second)
	for {
		select {
		case <-ticker1.C:
			t.Fatal("connect peer timeout")
			return
		default:
			time.Sleep(1 * time.Millisecond)
		}
		count := node1.node.streamManager.PeerCounts()
		if count < 1 {
			continue
		}
		break
	}
}

func setPovStatus(cc *context.ChainContext, t *testing.T) {
	l := ledger.NewLedger(cc.ConfigFile())
	block, td := mock.GeneratePovBlock(nil, 0)
	if err := l.AddPovBlock(block, td); err != nil {
		t.Fatal(err)
	}
	if err := l.AddPovBestHash(block.GetHeight(), block.GetHash()); err != nil {
		t.Fatal(err)
	}
	if err := l.SetPovLatestHeight(block.GetHeight()); err != nil {
		t.Fatal(err)
	}
	cc.EventBus().Publish(topic.EventPovSyncState, topic.SyncDone)
}
