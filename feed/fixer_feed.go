package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	. "github.com/korobool/btcticker/product"
)

var (
	fixerEurUri = "http://api.fixer.io/latest?base=EUR"
)

type FixerMsgLatest struct {
	Base  string
	Date  string
	Rates map[string]float64
}

type FixerFeed struct {
	BaseFeed
	client       *http.Client
	interrupt    chan struct{}
	wait         chan struct{}
	pollInterval time.Duration
	reqTiemout   time.Duration
}

func NewFixerFeed(agr *Aggregator) (Feed, error) {

	transport := &http.Transport{
		//TLSHandshakeTimeout
		//ResponseHeaderTimeout
		DisableKeepAlives: false,
	}

	client := &http.Client{Transport: transport}

	return &FixerFeed{
		BaseFeed: BaseFeed{
			Info:       FeedInfo{ProductEurUsd, "fixer"},
			aggregator: agr,
		},
		client:       client,
		interrupt:    make(chan struct{}),
		wait:         make(chan struct{}),
		pollInterval: 5 * time.Second,
		reqTiemout:   1 * time.Second,
	}, nil
}

func (f *FixerFeed) Run() error {
	f.aggregator.regFeed <- f.Info

	//f.client.Transport.Close()

	go func() {
		ticker := time.NewTicker(f.pollInterval)

		defer func() {
			ticker.Stop()
			f.aggregator.deregFeed <- f.Info
			close(f.wait)
		}()

		for {
			select {
			case <-ticker.C:
				rate, err := f.requestLatest(f.reqTiemout)
				if err != nil {
					log.Printf("fixer: %v", err)
					return
				}
				ts := time.Now().Unix()

				f.aggregator.tickMsgQueue <- &TickMsg{
					Info:     f.Info,
					Ts:       ts,
					TsBuy:    ts,
					BidPrice: rate,
				}
			case <-f.interrupt:
				log.Printf("fixer: push interrupted")
				return
			}
		}
	}()

	return nil
}

func (f *FixerFeed) Stop() error {
	close(f.interrupt)
	<-f.wait
	return nil
}

func (f *FixerFeed) Wait() chan struct{} {
	return f.wait
}

func (f *FixerFeed) GetInfo() FeedInfo {
	return f.Info
}

func (f *FixerFeed) GetName() string {
	return f.Info.Name
}

func (f *FixerFeed) requestLatest(timeout time.Duration) (float64, error) {

	var rate float64

	ctx, _ := context.WithTimeout(context.TODO(), timeout)
	req, err := http.NewRequest("GET", fixerEurUri, nil)
	if err != nil {
		return rate, err
	}

	resp, err := f.client.Do(req.WithContext(ctx))
	if err != nil {
		return rate, err
	}

	if resp.StatusCode != http.StatusOK {
		return rate, fmt.Errorf("status code: %v", resp.StatusCode)
	}

	defer resp.Body.Close()

	var latest FixerMsgLatest

	if err := json.NewDecoder(resp.Body).Decode(&latest); err != nil {
		return rate, err
	}
	rate, ok := latest.Rates["USD"]
	if !ok {
		return rate, fmt.Errorf("no such currency")
	}

	return rate, nil
}
