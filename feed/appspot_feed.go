package feed

import (
	"time"

	. "github.com/korobool/btcticker/product"
)

var (
	appspotEurUri = "http://rate-exchange-1.appspot.com/currency?from=EUR&to=USD"
)

type AppspotMsgCurrency struct {
	To   string  `json:"to"`
	Form string  `json:"from"`
	Rate float64 `json:"rate"`
}

type AppspotFeed struct {
	RestFeed
}

func NewAppspotFeed(agr *Aggregator) (Feed, error) {

	pollInterval := 5 * time.Second
	reqTimeout := 1 * time.Second

	info := FeedInfo{ProductEurUsd, "appspot"}
	rf, err := NewRestFeed(info, agr, pollInterval, reqTimeout)
	if err != nil {
		return nil, err
	}
	f := &AppspotFeed{RestFeed: *rf}
	f.SetPollHandler(f.poll)

	return f, nil
}

func (f *AppspotFeed) poll() (*TickMsg, error) {

	var msg AppspotMsgCurrency

	err := f.requestGet(appspotEurUri, &msg, f.reqTimeout)
	if err != nil {
		return nil, err
	}

	ts := time.Now().Unix()

	return &TickMsg{
		Info:     f.GetInfo(),
		Ts:       ts,
		TsBuy:    ts,
		BidPrice: msg.Rate,
	}, nil
}
