package feed

import (
	. "github.com/korobool/btcticker/product"
	"log"
	"time"
)

type FakeFeed struct {
	BaseFeed
	interrupt chan struct{}
	wait      chan struct{}
}

func NewFakeEurUsdFeed(agr *Aggregator) (Feed, error) {

	return &FakeFeed{
		BaseFeed: BaseFeed{
			Info:       FeedInfo{ProductEurUsd, "fake_eurusd"},
			aggregator: agr,
		},
		interrupt: make(chan struct{}),
		wait:      make(chan struct{}),
	}, nil
}

func (f *FakeFeed) Run() error {
	f.aggregator.regFeed <- f.Info

	go func() {
		ticker := time.NewTicker(2 * time.Second)

		defer func() {
			ticker.Stop()
			f.aggregator.deregFeed <- f.Info
			close(f.wait)
		}()

		for {
			select {
			case <-ticker.C:
				f.aggregator.tickMsgQueue <- &TickMsg{
					Info:     f.Info,
					Ts:       time.Now().Unix(),
					TsSell:   time.Now().Unix(),
					TsBuy:    time.Now().Unix() - 1,
					AskPrice: 1.05,
					BidPrice: 1.06,
				}
			case <-f.interrupt:
				log.Printf("fake push interrupted")
				return
			}
		}
	}()

	return nil
}

func (f *FakeFeed) Stop() error {
	close(f.interrupt)
	<-f.wait
	return nil
}

func (f *FakeFeed) Wait() chan struct{} {
	return f.wait
}

func (f *FakeFeed) GetInfo() FeedInfo {
	return f.Info
}

func (f *FakeFeed) GetName() string {
	return f.Info.Name
}
