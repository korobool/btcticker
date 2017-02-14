package feed

import (
	"fmt"

	. "github.com/korobool/btcticker/product"
)

type Feed interface {
	GetName() string
	GetInfo() FeedInfo
	Run() error
	Stop() error
	Wait() chan struct{}
}

type FeedConstructor func(*Aggregator) (Feed, error)

type FeedInfo struct {
	Product ProductType
	Name    string
}

func (i FeedInfo) String() string {
	return fmt.Sprintf("%s[%s]", i.Name, i.Product)
}

type TickMsg struct {
	Info      FeedInfo
	PriceSell float64
	PriceBuy  float64
	AskPrice  float64
	BidPrice  float64
	TsSell    int64
	TsBuy     int64
	Ts        int64
}

func (t TickMsg) String() string {
	return fmt.Sprintf("%s %d ask:%.4f bid:%.4f trade_s:%.4f/%d trade_b:%.4f/%d",
		t.Info, t.Ts, t.AskPrice, t.BidPrice, t.PriceSell, t.TsSell, t.PriceBuy, t.TsBuy)
}

type BaseFeed struct {
	Info       FeedInfo
	aggregator *Aggregator
}
