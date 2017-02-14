package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// RestFeed is a generic base 'class' for basic http REST-polling feed sources
type RestFeed struct {
	BaseFeed
	client *http.Client
	// Closing interrupt channel starts gracefull shutdown
	interrupt chan struct{}
	// wait channel will be closed when feed can be destroyed safelly
	wait chan struct{}
	// Handler which will be invoked on each poll round
	pollHandler func() (*TickMsg, error)
	// Interval for requesting remote API
	pollInterval time.Duration
	// Timout for http request
	reqTimeout time.Duration
}

func NewRestFeed(info FeedInfo, agr *Aggregator, pollInterval, reqTimeout time.Duration) (*RestFeed, error) {

	transport := &http.Transport{
		//TLSHandshakeTimeout
		//ResponseHeaderTimeout
		DisableKeepAlives: false,
	}

	client := &http.Client{Transport: transport}

	return &RestFeed{
		BaseFeed: BaseFeed{
			Info:       info,
			aggregator: agr,
		},
		client:       client,
		interrupt:    make(chan struct{}),
		wait:         make(chan struct{}),
		pollHandler:  func() (*TickMsg, error) { return nil, nil },
		pollInterval: pollInterval,
		reqTimeout:   reqTimeout,
	}, nil
}

func (f *RestFeed) Run() error {
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
				msg, err := f.pollHandler()
				if err != nil {
					log.Printf("%s: %v", f.GetName(), err)
					return
				}
				if msg != nil {
					f.aggregator.tickMsgQueue <- msg
				}
			case <-f.interrupt:
				log.Printf("%s: push interrupted", f.GetName())
				return
			}
		}
	}()

	return nil
}

func (f *RestFeed) Stop() error {
	close(f.interrupt)
	<-f.wait
	return nil
}

func (f *RestFeed) Wait() chan struct{} {
	return f.wait
}

func (f *RestFeed) GetInfo() FeedInfo {
	return f.Info
}

func (f *RestFeed) GetName() string {
	return f.Info.Name
}

// SetPollHandler() sets handler which will be invoked on each polling round
func (f *RestFeed) SetPollHandler(h func() (*TickMsg, error)) {
	f.pollHandler = h
}

// requestGet() is a helper for requesting remote API (http GET)
func (f *RestFeed) requestGet(url string, jsonData interface{}, timeout time.Duration) error {

	ctx, _ := context.WithTimeout(context.TODO(), timeout)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := f.client.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code: %v", resp.StatusCode)
	}

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&jsonData); err != nil {
		return err
	}

	return nil
}
