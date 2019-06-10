package consensus

import (
	"github.com/qlcchain/go-qlc/common/event"
	"github.com/qlcchain/go-qlc/config"
	"github.com/qlcchain/go-qlc/ledger"
	"github.com/qlcchain/go-qlc/ledger/process"
	"github.com/qlcchain/go-qlc/log"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type ConsensusAlgorithm interface {
	Init()
	Start()
	Stop()
	ProcessMsg(bs *BlockSource)
}

type Consensus struct {
	ca       ConsensusAlgorithm
	recv     *Receiver
	logger   *zap.SugaredLogger
	ledger   *ledger.Ledger
	verifier *process.LedgerVerifier
}

var GlobalUncheckedBlockNum atomic.Uint64

func NewConsensus(ca ConsensusAlgorithm, cfg *config.Config, eb event.EventBus) *Consensus {
	l := ledger.NewLedger(cfg.LedgerDir())

	return &Consensus{
		ca:       ca,
		recv:     NewReceiver(eb),
		logger:   log.NewLogger("consensus"),
		ledger:   l,
		verifier: process.NewLedgerVerifier(l),
	}
}

func (c *Consensus) Init() {
	c.ca.Init()
	c.recv.init(c)
}

func (c *Consensus) Start() {
	go c.ca.Start()

	err := c.recv.start()
	if err != nil {
		c.logger.Error("receiver start failed")
	}
}

func (c *Consensus) Stop() {
	err := c.recv.stop()
	if err != nil {
		c.logger.Error("receiver stop failed")
	}

	c.ca.Stop()
}
