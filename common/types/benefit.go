package types

import (
	"encoding/json"
)

//go:generate msgp
type Benefit struct {
	Balance Balance `msg:"balance,extension" json:"balance"`
	Vote    Balance `msg:"vote,extension" json:"vote"`
	Network Balance `msg:"network,extension" json:"network"`
	Storage Balance `msg:"storage,extension" json:"storage"`
	Oracle  Balance `msg:"oracle,extension" json:"oracle"`
	Total   Balance `msg:"total,extension" json:"total"`
}

func (b *Benefit) String() string {
	bytes, _ := json.Marshal(b)
	return string(bytes)
}

func (b *Benefit) Serialize() ([]byte, error) {
	return b.MarshalMsg(nil)
}

func (b *Benefit) Deserialize(text []byte) error {
	_, err := b.UnmarshalMsg(text)
	if err != nil {
		return err
	}
	return nil
}

func (b *Benefit) Clone() *Benefit {
	clone := Benefit{}
	bytes, _ := b.Serialize()
	_ = clone.Deserialize(bytes)
	return &clone
}

var (
	// ZeroBalance zero
	ZeroBenefit = &Benefit{
		Vote:    ZeroBalance,
		Network: ZeroBalance,
		Storage: ZeroBalance,
		Oracle:  ZeroBalance,
		Balance: ZeroBalance,
		Total:   ZeroBalance,
	}
)
