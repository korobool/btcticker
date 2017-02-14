package feed

import (
	"fmt"
	"log"
	"time"

	. "github.com/korobool/btcticker/product"
)

type AgrTickMsg struct {
	Product       ProductType
	PriceSell     float64
	PriceBuy      float64
	TsSell        int64
	TsBuy         int64
	AskPrice      float64
	BidPrice      float64
	ActiveSources int
	TotalSources  int
}

func (t AgrTickMsg) String() string {
	return fmt.Sprintf("%s[%d/%d] ask:%.4f bid:%.4f trade_s:%.4f/%d trade_b:%.4f/%d", t.Product, t.ActiveSources, t.TotalSources, t.AskPrice, t.BidPrice, t.PriceSell, t.TsSell, t.PriceBuy, t.TsBuy)
}

// Aggregator mananges starting/stopping feed source listeners/pollers.
// Selects best prices from active sources per each product and sends
// aggregated message to upstream.
type Aggregator struct {
	// Map of the last message for each "product-feed" pair
	feeds map[ProductType]map[string]*TickMsg
	// Feed must use regFeed channel to register itself
	regFeed chan FeedInfo
	// Feed must use deregFeed channel to deregister itself
	deregFeed chan FeedInfo
	// Writing to ForecTick forces aggreagtor to emit tick
	// for specified product
	ForceTick chan ProductType
	// wait channel will be closed when aggregator can be destroyed safelly
	wait chan struct{}
	// Aggregator starts gracefull shutdown when Done channel is closed
	Done chan struct{}
	// Tick is outbound queue of aggregated messages
	Tick chan *AgrTickMsg
	// Inbound queue of messages from the feeds
	tickMsgQueue chan *TickMsg
	// Number of enabled feed sources per product
	totalSources map[ProductType]int
	// Current number of active feed sources per product
	activeSources map[ProductType]int
	// Feed restart retry interval
	retryInterval time.Duration
}

// NewAggregator creates new instance of Aggregator.
// When passed done channel is closed aggregator will start
// graceful shutdown.
func NewAggregator(done chan struct{}) (*Aggregator, error) {
	agr := Aggregator{
		wait:          make(chan struct{}),
		tickMsgQueue:  make(chan *TickMsg, 32),
		Tick:          make(chan *AgrTickMsg, 32),
		regFeed:       make(chan FeedInfo),
		deregFeed:     make(chan FeedInfo),
		ForceTick:     make(chan ProductType),
		Done:          done,
		feeds:         make(map[ProductType]map[string]*TickMsg),
		totalSources:  make(map[ProductType]int),
		activeSources: make(map[ProductType]int),
		retryInterval: 2 * time.Second,
	}
	return &agr, nil
}

// Run() starts each feed in a separate goroutine (feed runner).
// Listens channels to:
// - external interrupt
// - register/deregister feeds
// - update last message for each "product-feed" pair
// - based on updated last message emits new aggregated message
// - emit aggregated tick message by force
func (a *Aggregator) Run(feedNameList []string) {

	defer func() {
		close(a.Tick)
		close(a.wait)
	}()

	a.launchFeedRunners(feedNameList)

	shouldExit := false
	for {
		select {
		case info := <-a.regFeed:
			a.registerFeed(&info)
		case info := <-a.deregFeed:
			a.deregisterFeed(&info)
		case prod := <-a.ForceTick:
			a.emitTick(prod)
		case <-a.Done:
			if !shouldExit {
				shouldExit = true
			} else if len(a.feeds) == 0 {
				log.Printf("aggregator: exiting")
				return
			}
		case msg := <-a.tickMsgQueue:
			if a.updateFeedTick(msg) {
				a.emitTick(msg.Info.Product)
			}
		}
	}
}

func (a *Aggregator) Wait() {
	<-a.wait
}

// launchFeedRunners() initialises instance for each feed name
// from []feedNameList slice. If []feedNameList is nil
// all feeds from FeedRegistry will be started.
func (a *Aggregator) launchFeedRunners(feedNameList []string) {
	if len(feedNameList) == 0 {
		for info, feedConstructor := range FeedRegistry {
			a.totalSources[info.Product] += 1
			go a.runFeed(feedConstructor, a.retryInterval)
		}
	} else {
		revRegistry := make(map[string]FeedInfo)
		for info, _ := range FeedRegistry {
			revRegistry[info.Name] = info
		}
		for _, feedName := range feedNameList {
			if info, ok := revRegistry[feedName]; ok {
				if feedConstructor, ok := FeedRegistry[info]; ok {
					a.totalSources[info.Product] += 1
					go a.runFeed(feedConstructor, a.retryInterval)
				}
			}
		}
	}
}

func (a *Aggregator) registerFeed(info *FeedInfo) {
	log.Printf("aggregator: registering feed: %v", *info)

	if _, ok := a.feeds[info.Product]; !ok {
		a.feeds[info.Product] = make(map[string]*TickMsg)
	}
	a.feeds[info.Product][info.Name] = nil
}

func (a *Aggregator) deregisterFeed(info *FeedInfo) {
	log.Printf("aggregator: de-registering feed: %v", *info)

	if prodFeeds, ok := a.feeds[info.Product]; ok {
		if _, ok := prodFeeds[info.Name]; ok {
			delete(prodFeeds, info.Name)

			a.emitTick(info.Product)

			if len(prodFeeds) == 0 {
				delete(a.feeds, info.Product)
			}
		}
	}
}

func (a *Aggregator) updateFeedTick(msg *TickMsg) bool {
	if prodFeeds, ok := a.feeds[msg.Info.Product]; ok {
		if _, ok := prodFeeds[msg.Info.Name]; ok {
			prodFeeds[msg.Info.Name] = msg
			return true
		}
	}
	return false
}

// emitTick() calculates best bid(maximum) price, best ask(minimum)
// price and current active feed sources for specified product.
// Then emits AgrTickMsg message.
func (a *Aggregator) emitTick(product ProductType) *AgrTickMsg {

	if total, ok := a.totalSources[product]; ok {
		if prodFeeds, ok := a.feeds[product]; ok {

			active := 0
			var minAskPrice, maxBidPrice float64

			for _, msg := range prodFeeds {
				if msg != nil {
					active += 1
					if msg.Ts > 0 {
						if msg.BidPrice > maxBidPrice {
							maxBidPrice = msg.BidPrice
						}
						if minAskPrice == 0 || msg.AskPrice < minAskPrice {
							minAskPrice = msg.AskPrice
						}
					}
				}
			}
			if active > 0 {
				agrMsg := &AgrTickMsg{
					Product:       product,
					AskPrice:      minAskPrice,
					BidPrice:      maxBidPrice,
					ActiveSources: active,
					TotalSources:  total,
				}

				log.Printf("emitTick: %v", *agrMsg)

				a.Tick <- agrMsg

				return agrMsg
			}
		}
	}
	return nil
}

// runFeed() runs feed and restarts it when feed fails for some reason.
// Feeds are restarted with retryInterval delay.
func (a *Aggregator) runFeed(constructor FeedConstructor, retryInterval time.Duration) {

	retryWait := func() {
		<-time.After(retryInterval)
	}

	for {
		source, err := constructor(a)
		if err != nil {
			log.Printf("failed to initialize feed %v", err)
			retryWait()
			continue
		}
		if err := source.Run(); err != nil {
			log.Printf("failed to start feed %s: %v", source.GetName(), err)
			retryWait()
			continue
		}
		select {
		case <-source.Wait():
			retryWait()
			log.Printf("restarting feed: %s", source.GetName())
			continue
		case <-a.Done:
			log.Printf("stopping feed: %s", source.GetName())
			source.Stop()
			return
		}
	}
}
