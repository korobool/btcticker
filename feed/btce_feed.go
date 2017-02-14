package feed

import (
	"time"

	. "github.com/korobool/btcticker/product"
)

var (
	btceTickerUri = "https://btc-e.com/api/3/ticker/btc_usd"
)

type BtceMsgTicker struct {
	Product struct {
		High    float64 `json:"high"`
		Low     float64 `json:"low"`
		Avg     float64 `json:"avg"`
		Vol     float64 `json:"vol"`
		VolCur  float64 `json:"vol_cur"`
		Last    float64 `json:"last"`
		Buy     float64 `json:"buy"`
		Sell    float64 `json:"sell"`
		Updated int64   `json:"updated"`
	} `json:"btc_usd"`
}

type BtceFeed struct {
	RestFeed
	ts int64
}

func NewBtceFeed(agr *Aggregator) (Feed, error) {
	pollInterval := 2 * time.Second
	reqTimeout := 1 * time.Second

	info := FeedInfo{ProductBtcUsd, "btce"}
	rf, err := NewRestFeed(info, agr, pollInterval, reqTimeout)
	if err != nil {
		return nil, err
	}
	f := &BtceFeed{RestFeed: *rf}
	f.SetPollHandler(f.poll)

	return f, nil
}

func (f *BtceFeed) poll() (*TickMsg, error) {

	var msg BtceMsgTicker

	err := f.requestGet(btceTickerUri, &msg, f.reqTimeout)
	if err != nil {
		return nil, err
	}

	ts := msg.Product.Updated

	if ts > f.ts {
		f.ts = ts
		return &TickMsg{
			Info:     f.GetInfo(),
			Ts:       ts,
			AskPrice: msg.Product.Sell,
			BidPrice: msg.Product.Buy,
		}, nil
	}
	return nil, nil
}
