package feed

import (
	. "github.com/demyan/btcticker/product"
	"log"
	"time"
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

type Aggregator struct {
	feeds map[ProductType]map[string]*TickMsg
	// Register feed source
	regFeed chan FeedInfo
	// Deregister feed source
	deregFeed chan FeedInfo
	// Force aggreagtor to emit tick
	ForceTick chan ProductType
	// Shutdown self
	Done chan struct{}
	//
	Tick chan *AgrTickMsg
	// Inbound messages from the feeds
	tickMsgQueue chan *TickMsg
	//
	totalSources  map[ProductType]int
	activeSources map[ProductType]int
	// Feed start retry interval
	retryInterval time.Duration
}

func NewAggregator() (*Aggregator, error) {
	agr := Aggregator{
		tickMsgQueue:  make(chan *TickMsg, 32),
		Tick:          make(chan *AgrTickMsg, 32),
		regFeed:       make(chan FeedInfo),
		deregFeed:     make(chan FeedInfo),
		ForceTick:     make(chan ProductType),
		Done:          make(chan struct{}),
		feeds:         make(map[ProductType]map[string]*TickMsg),
		totalSources:  make(map[ProductType]int),
		activeSources: make(map[ProductType]int),
		retryInterval: 2 * time.Second,
	}
	return &agr, nil
}

func (a *Aggregator) Run(feedNameList []string) {

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
				return
			}
		case msg := <-a.tickMsgQueue:
			if a.updateFeedTick(msg) {
				a.emitTick(msg.Info.Product)
			}
		}
	}
}

func (a *Aggregator) launchFeedRunners(feedNameList []string) {
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

				log.Printf("emitTick: %v", agrMsg)

				a.Tick <- agrMsg

				return agrMsg
			}
		}
	}
	return nil
}

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
