package feed

import (
	. "github.com/korobool/btcticker/product"
)

var FeedRegistry map[FeedInfo]FeedConstructor

func init() {
	FeedRegistry = map[FeedInfo]FeedConstructor{
		{ProductBtcUsd, "gdax"}:        NewGdaxWebSocketFeed,
		{ProductBtcUsd, "btce"}:        NewBtceFeed,
		{ProductEurUsd, "fixer"}:       NewFixerFeed,
		{ProductEurUsd, "appspot"}:     NewAppspotFeed,
		{ProductEurUsd, "fake_eurusd"}: NewFakeEurUsdFeed,
	}
}
