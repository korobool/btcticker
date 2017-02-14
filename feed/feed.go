package feed

import (
	"fmt"

	. "github.com/korobool/btcticker/product"
)

// Feed source interface
type Feed interface {
	// human-readable string name for feed source
	GetName() string
	// FeedInfo about feed source
	GetInfo() FeedInfo
	// run feed (non-blockable)
	Run() error
	// signals feed to exit
	Stop() error
	// feed source should block here until it's safe to exit
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

// TickMsg contains update message from feed source
// which should be propagated to aggregator
type TickMsg struct {
	Info FeedInfo
	// Last price for sell trade
	PriceSell float64
	// Last price for buy trade
	PriceBuy float64
	// Best ask price
	AskPrice float64
	// Best bid price
	BidPrice float64
	// Last timestamp for sell trade
	TsSell int64
	// Last timestamp for buy trade
	TsBuy int64
	// Current timestamp for message
	Ts int64
}

func (t TickMsg) String() string {
	return fmt.Sprintf("%s %d ask:%.4f bid:%.4f trade_s:%.4f/%d trade_b:%.4f/%d",
		t.Info, t.Ts, t.AskPrice, t.BidPrice, t.PriceSell, t.TsSell, t.PriceBuy, t.TsBuy)
}

// BaseFeed is base 'class' for all feed sources
type BaseFeed struct {
	Info       FeedInfo
	aggregator *Aggregator
}
