package feed

import (
	. "github.com/demyan/btcticker/product"
)

type Feed interface {
	GetName() string
	GetInfo() FeedInfo
	Run() error
	Stop() error
	Wait() chan struct{}
}

type FeedConstructor func(*Aggregator) (Feed, error)

var FeedRegistry map[FeedInfo]FeedConstructor

type FeedInfo struct {
	Product ProductType
	Name    string
}

func init() {
	FeedRegistry = map[FeedInfo]FeedConstructor{
		{ProductBtcUsd, "gdax"}:        NewGdaxWebSocketFeed,
		{ProductEurUsd, "fake_eurusd"}: NewFakeEurUsdFeed,
	}
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

type BaseFeed struct {
	Info       FeedInfo
	aggregator *Aggregator
}
