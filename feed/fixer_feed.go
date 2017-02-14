package feed

import (
	"fmt"
	"time"

	. "github.com/korobool/btcticker/product"
)

var (
	fixerEurUri = "http://api.fixer.io/latest?base=EUR"
)

type FixerMsgLatest struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
}

type FixerFeed struct {
	RestFeed
}

func NewFixerFeed(agr *Aggregator) (Feed, error) {

	pollInterval := 5 * time.Second
	reqTimeout := 1 * time.Second

	info := FeedInfo{ProductEurUsd, "fixer"}
	rf, err := NewRestFeed(info, agr, pollInterval, reqTimeout)
	if err != nil {
		return nil, err
	}
	f := &FixerFeed{RestFeed: *rf}
	f.SetPollHandler(f.poll)

	return f, nil
}

func (f *FixerFeed) poll() (*TickMsg, error) {

	var msg FixerMsgLatest

	err := f.requestGet(fixerEurUri, &msg, f.reqTimeout)
	if err != nil {
		return nil, err
	}
	rate, ok := msg.Rates["USD"]
	if !ok {
		return nil, fmt.Errorf("no such currency")
	}

	ts := time.Now().Unix()

	return &TickMsg{
		Info:     f.GetInfo(),
		Ts:       ts,
		TsBuy:    ts,
		BidPrice: rate,
	}, nil
}
