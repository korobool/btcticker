package feed

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	. "github.com/korobool/btcticker/product"
)

var (
	gdaxMsgSubscribe = []byte(`{"type":"subscribe","product_ids":["BTC-USD"]}`)
	gdaxWsUrl        = "wss://ws-feed.gdax.com"
	gdaxRestBookUrl  = "https://api.gdax.com/products/BTC-USD/book?level=3"
)

// Aggregated GDAX Websocket API message stucture with only important fields
type MsgGdax struct {
	Type      string     `json:"type"`                 // "match" | "error" | "heartbeat"
	Sequence  *int64     `json:"sequence,omitempty"`   // 50
	OrderId   *string    `json:"order_id,omitempty"`   // "d50ec984-77a8-460a-b958-66f114b0de9b"
	Time      *time.Time `json:"time,omitempty"`       // "2014-11-07T08:19:27.028459Z"
	ProductId *string    `json:"product_id,omitempty"` // "BTC-USD"
	Size      *string    `json:"size,omitempty"`       // "5.23512"
	Price     *string    `json:"price,omitempty"`      // "400.23",
	Side      *string    `json:"side,omitempty"`       // "sell"
	Message   *string    `json:"message,omitempty"`    // "error message"
}

// REST order book data
type MsgGdaxOrderBook struct {
	Asks     [][]string `json:"asks"`
	Bids     [][]string `json:"bids"`
	Sequence int64      `json:"sequence"`
}

type OrderBook struct {
	asks map[string]float64
	bids map[string]float64
}

func newOrderBook() *OrderBook {
	return &OrderBook{
		asks: make(map[string]float64),
		bids: make(map[string]float64),
	}
}

func (b *OrderBook) AddBid(orderId string, price float64) {
	b.bids[orderId] = price
}

func (b *OrderBook) AddAsk(orderId string, price float64) {
	b.asks[orderId] = price
}

func (b *OrderBook) DeleteBid(orderId string) bool {
	_, ok := b.bids[orderId]
	if ok {
		delete(b.bids, orderId)
	}
	return ok
}

func (b *OrderBook) DeleteAsk(orderId string) bool {
	_, ok := b.asks[orderId]
	if ok {
		delete(b.asks, orderId)
	}
	return ok
}

// GDAX websocket feed source
type GdaxWebSocketFeed struct {
	Info FeedInfo
	// signal for external interrupt
	interrupt chan struct{}
	// wait channel will be closed when instance ready to be destroyed safelly
	wait          chan struct{}
	initOrderBook chan struct{}
	pendingMsgs   [][]byte

	wg         *sync.WaitGroup
	aggregator *Aggregator

	orderBook *OrderBook
	// last message sequence number
	// (messages with older sequence number must be ignored)
	sequence int64
	// last sell trade timestamp
	tsSell int64
	// last buy trade timestamp
	tsBuy int64
	// last sell trade price
	priceSell float64
	// last buy trade price
	priceBuy float64
	// best ask price (min)
	askPrice float64
	// best bid price (max)
	bidPrice float64

	conn *websocket.Conn

	// deadline timeout for websocket read operations
	TimeoutRead time.Duration
	// deadline timeout for websocket write operations
	TimeoutWrite time.Duration
	// period to send websocket ping messages
	pingPeriod time.Duration
}

//func NewGdaxWebSocketFeed(agr *Aggregator, wsUrl string, wsHeaders http.Header) (*GdaxWebSocketFeed, error) {
func NewGdaxWebSocketFeed(agr *Aggregator) (Feed, error) {
	var wsHeaders http.Header

	var dialer websocket.Dialer

	conn, _, err := dialer.Dial(gdaxWsUrl, wsHeaders)
	if err != nil {
		return nil, err
	}

	return &GdaxWebSocketFeed{
		Info:          FeedInfo{ProductBtcUsd, "gdax"},
		interrupt:     make(chan struct{}),
		wait:          make(chan struct{}),
		initOrderBook: make(chan struct{}),
		pendingMsgs:   make([][]byte, 0, 100),
		wg:            new(sync.WaitGroup),
		orderBook:     newOrderBook(),
		aggregator:    agr,
		conn:          conn,
		TimeoutRead:   2 * time.Second,
		TimeoutWrite:  2 * time.Second,
		pingPeriod:    1 * time.Second,
	}, nil
}

// Run registers feed source with aggreator.
// Starts pull and push goroutines in background.
func (f *GdaxWebSocketFeed) Run() error {
	f.aggregator.regFeed <- f.Info

	f.wg.Add(2)
	// Spawn goroutine which will close wait channel
	// (signal that both recv/send goroutines were stopped)
	go func() {
		f.wg.Wait()
		f.aggregator.deregFeed <- f.Info
		close(f.wait)
	}()
	go f.pull()
	go f.push()

	return nil
}

// Send "subscribe" message
func (f *GdaxWebSocketFeed) Subscribe() error {
	// https://docs.gdax.com/?python#subscribe
	//{
	//"type": "subscribe",
	//"product_ids": [
	//    "BTC-USD",
	//]
	//}
	if err := f.conn.WriteMessage(websocket.TextMessage, gdaxMsgSubscribe); err != nil {
		return err
	}
	if err := f.getOrderBook(); err != nil {
		return err
	}
	close(f.initOrderBook)

	return nil
}

func (f *GdaxWebSocketFeed) Stop() error {
	close(f.interrupt)
	<-f.wait

	return nil
}

func (f *GdaxWebSocketFeed) Wait() chan struct{} {
	return f.wait
}

func (f *GdaxWebSocketFeed) Close() {
	f.conn.Close()
}

func (f *GdaxWebSocketFeed) GetInfo() FeedInfo {
	return f.Info
}

func (f *GdaxWebSocketFeed) GetName() string {
	return f.Info.Name
}

func (f *GdaxWebSocketFeed) enableHeartbeat() {
	// https://docs.gdax.com/?python#heartbeat
	//{
	//"type": "heartbeat",
	//"on": true
	//}
	return
}

// Pull goroutine handels inbound messages from websocket.
// It stores inbound messages in temporary queue till orderbook will be initialised.
// When orderbook initialised messages wil be processed and written to tickMsgQueue.
func (f *GdaxWebSocketFeed) pull() {
	defer func() {
		f.Close()
		f.wg.Done()
	}()

	// Setting pong handler will expand read deadline on each pong.
	deadlineHandler := func(string) error {
		deadline := time.Now().Add(f.TimeoutRead)
		return f.conn.SetReadDeadline(deadline)
	}
	if err := deadlineHandler(""); err != nil {
		return
	}
	f.conn.SetPongHandler(deadlineHandler)

	for {
		msgType, msg, err := f.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("gdax: pull error: %v", err)
			} else {
				log.Printf("gdax: pull gone: %v", err)
			}
			return
		}

		if msgType == websocket.TextMessage {
			select {
			// initOrderBook channel is closed whan orderbook is initialised.
			case <-f.initOrderBook:
			default:
				f.pendingMsgs = append(f.pendingMsgs, msg)
				continue
			}

			if len(f.pendingMsgs) > 0 {
				log.Printf("gdax: loading pending: %d", len(f.pendingMsgs))

				// Processing all pending messages
				for _, pendingMsg := range f.pendingMsgs {
					feedMsg, err := f.processMsg(pendingMsg)
					if err != nil {
						log.Printf("gdax: processMsg: %v", err)
					} else if feedMsg != nil {
						// Sending message with updated 'tick'
						f.aggregator.tickMsgQueue <- feedMsg
					}
				}
				f.pendingMsgs = [][]byte{}
			}

			feedMsg, err := f.processMsg(msg)
			if err != nil {
				log.Printf("gdax: processMsg: %v", err)

			} else if feedMsg != nil {
				// Sending message with updated 'tick'
				f.aggregator.tickMsgQueue <- feedMsg
			}
		}
	}
}

// Push goroutine handels outbound messages to websocket.
// Send ping messages with pingPeriod period.
func (f *GdaxWebSocketFeed) push() {
	ticker := time.NewTicker(f.pingPeriod)

	defer func() {
		ticker.Stop()
		f.Close()
		f.wg.Done()
	}()

	if err := f.Subscribe(); err != nil {
		log.Printf("gdax: push failed to subscribe: %v", err)
		return
	}

	for {
		select {
		case <-ticker.C:
			f.conn.SetWriteDeadline(time.Now().Add(f.TimeoutWrite))
			err := f.conn.WriteMessage(websocket.PingMessage, []byte{})
			if err != nil {
				log.Printf("gdax: push error: %v", err)
				return
			}
		case <-f.interrupt:
			log.Printf("gdax: push interrupted")
			return
		}
	}
}

// Fetch process current orderbook state via REST API.
func (f *GdaxWebSocketFeed) getOrderBook() error {
	req, err := http.NewRequest("GET", gdaxRestBookUrl, nil)
	if err != nil {
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	var orderBook MsgGdaxOrderBook

	if err := json.NewDecoder(resp.Body).Decode(&orderBook); err != nil {
		return err
	}

	f.processOrderBook(&orderBook)

	return nil
}

// Process initial orderbook.
// Look for min ask price and max bid price.
func (f *GdaxWebSocketFeed) processOrderBook(orderBookMsg *MsgGdaxOrderBook) {

	log.Printf("gdax order book: %d/%d (asks/bids)",
		len(orderBookMsg.Asks), len(orderBookMsg.Bids))

	var askPrice, bidPrice float64

	for _, row := range orderBookMsg.Asks {
		if len(row) == 3 {
			price, err := strconv.ParseFloat(row[0], 64)
			if err != nil {
				continue
			}
			f.orderBook.AddAsk(row[2], price)

			if askPrice == 0 || price < askPrice {
				askPrice = price
			}
		}
	}

	for _, row := range orderBookMsg.Bids {
		if len(row) == 3 {
			price, err := strconv.ParseFloat(row[0], 64)
			if err != nil {
				continue
			}
			f.orderBook.AddBid(row[2], price)

			if price > bidPrice {
				bidPrice = price
			}
		}
	}
	// Store state of current best ask/bid and message sequence.
	f.sequence = orderBookMsg.Sequence
	f.askPrice = askPrice
	f.bidPrice = bidPrice
}

// Recalculate best ask/bid prices (min/max) on orderbook update.
func (f *GdaxWebSocketFeed) recalcBestAskBid() {
	var askPrice, bidPrice float64

	for _, price := range f.orderBook.asks {
		if askPrice == 0 || price < askPrice {
			askPrice = price
		}
	}
	for _, price := range f.orderBook.bids {
		if price > bidPrice {
			bidPrice = price
		}
	}
	f.askPrice, f.bidPrice = askPrice, bidPrice
}

// Processes messages from websocket stream.
// Handles "error", "done", "received" and "match" messages:
// - "received" - order added to orderbook
// - "match" - trade event (sell or buy)
// - "done" - order removed from orderbook
// Updates orderbook based on "received" and "done" messages.
// Stores last sell/buy price and timestamp based on "match" message.
// Builds new TickMsg if orderbook or last sell/buy price was updated.
func (f *GdaxWebSocketFeed) processMsg(msgData []byte) (*TickMsg, error) {

	var msg MsgGdax
	if err := json.Unmarshal(msgData, &msg); err != nil {
		return nil, err
	}
	if msg.Type == "error" {
		errMsg := "unknown error"
		if msg.Message != nil {
			errMsg = *msg.Message
		}
		return nil, fmt.Errorf("error msg: %s", errMsg)
	}

	if msg.Sequence == nil || msg.Time == nil || msg.Side == nil {
		return nil, nil
	}
	if *msg.Sequence <= f.sequence {
		return nil, nil
	}

	f.sequence = *msg.Sequence
	ts := msg.Time.Unix()

	var updatedAskBid, updatedLast bool

	if msg.Type == "done" && msg.OrderId != nil {
		if *msg.Side == "sell" {
			f.orderBook.DeleteAsk(*msg.OrderId)
		} else if *msg.Side == "buy" {
			f.orderBook.DeleteBid(*msg.OrderId)
		}
		updatedAskBid = true

	} else if msg.Price != nil {

		price, err := strconv.ParseFloat(*msg.Price, 64)
		if err != nil {
			return nil, nil
		}

		if msg.Type == "received" && msg.OrderId != nil {
			if *msg.Side == "sell" {
				f.orderBook.AddAsk(*msg.OrderId, price)

			} else if *msg.Side == "buy" {
				f.orderBook.AddBid(*msg.OrderId, price)

			}
			updatedAskBid = true

		} else if msg.Type == "match" && msg.Price != nil {

			log.Printf("gdax: match: %d %s %s %v",
				*msg.Sequence, *msg.Side, *msg.Price, *msg.Time)

			if *msg.Side == "sell" && ts >= f.tsSell {
				f.tsSell = ts
				f.priceSell = price
				updatedLast = true
			} else if *msg.Side == "buy" && ts >= f.tsBuy {
				f.tsBuy = ts
				f.priceBuy = price
				updatedLast = true
			}
		}
	}

	if updatedAskBid {
		f.recalcBestAskBid()
	}
	if updatedAskBid || updatedLast {
		return &TickMsg{
			Info:      f.Info,
			TsSell:    f.tsSell,
			TsBuy:     f.tsBuy,
			PriceSell: f.priceSell,
			PriceBuy:  f.priceBuy,
			AskPrice:  f.askPrice,
			BidPrice:  f.bidPrice,
			Ts:        ts,
		}, nil
	}

	return nil, nil
}
