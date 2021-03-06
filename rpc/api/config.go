package api

import (
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/qlcchain/go-qlc/chain/context"
	"github.com/qlcchain/go-qlc/common/event"
	"github.com/qlcchain/go-qlc/common/topic"
	"github.com/qlcchain/go-qlc/common/types"
	"github.com/qlcchain/go-qlc/config"
	"github.com/qlcchain/go-qlc/log"
)

const interval = 3 * 60

type ConfigApi struct {
	context    *context.ChainContext
	cfgManager *config.CfgManager
	eb         event.EventBus
	token      string
	mark       string
	updateTime int64
	logger     *zap.SugaredLogger
}

var (
	ErrIdentity  = errors.New("identity authentication failed")
	ErrMark      = errors.New("config is being modified by someone else")
	ErrOperation = errors.New("nothing has been modified")
)

func NewConfigApi(cfgFile string) *ConfigApi {
	cc := context.NewChainContext(cfgFile)
	cfgManager, _ := cc.ConfigManager()
	cfg, _ := cfgManager.Config()
	return &ConfigApi{
		context:    cc,
		eb:         cc.EventBus(),
		cfgManager: cfgManager,
		token:      cfg.Manager.AdminToken,
		logger:     log.NewLogger("rpc/config"),
	}
}

func (c *ConfigApi) CurrentConfig(token string) (*config.Config, error) {
	if token != c.token {
		return nil, ErrIdentity
	}
	return c.cfgManager.Config()
}

func (c *ConfigApi) Update(params []string, token string, mark string) (*config.Config, error) {
	if token != c.token {
		return nil, ErrIdentity
	}
	if mark != c.mark {
		now := time.Now().Unix()
		if now-c.updateTime > interval {
			c.cfgManager.Discard()
			c.mark = mark
			c.updateTime = time.Now().Unix()
		} else {
			return nil, ErrMark
		}
	} else {
		c.updateTime = time.Now().Unix()
	}
	return c.cfgManager.UpdateParams(params)
}

func (c *ConfigApi) Difference(token string, mark string) (string, error) {
	if token != c.token {
		return "", ErrIdentity
	}
	if mark != c.mark {
		return "", ErrOperation
	}
	return c.cfgManager.Diff()
}

func (c *ConfigApi) Commit(token string, mark string) (bool, error) {
	if token != c.token {
		return false, ErrIdentity
	}
	if mark != c.mark {
		return false, ErrOperation
	}
	c.eb.Publish(topic.EventRestartChain, types.NewTuple(c.cfgManager.ConfigFile, false))
	return true, nil
}

func (c *ConfigApi) Save(token string, mark string) (bool, error) {
	if token != c.token {
		return false, ErrIdentity
	}
	if mark != c.mark {
		return false, ErrOperation
	}
	c.eb.Publish(topic.EventRestartChain, types.NewTuple(c.cfgManager.ConfigFile, true))
	return true, nil
}
