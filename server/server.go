package server

import (
	"fmt"
	"net/http"

	"github.com/korobool/btcticker/feed"
	. "github.com/korobool/btcticker/product"
)

// Server operates with websocket clients (viewers).
// Listens for aggregated messages from aggreagtor,
// calculates values for ticker string an build it.
// Broadcasts updated ticker string to all clients.
type Server struct {
	// wait channel will be closed when instance ready to be destroyed safelly
	wait chan struct{}
	// aggregator
	aggregator *feed.Aggregator
	// Registered clients.
	clients map[*Client]struct{}
	// Register requests from the clients.
	register chan *Client
	// Deregister requests from clients.
	deregister chan *Client
	// Last prcies/timestamps per product
	lastState map[ProductType]*feed.AgrTickMsg
	// Cached ticker string
	cachedTick string
}

// Instantiate Server
func New(aggr *feed.Aggregator) *Server {
	return &Server{
		wait:       make(chan struct{}),
		aggregator: aggr,
		register:   make(chan *Client),
		deregister: make(chan *Client),
		clients:    make(map[*Client]struct{}),
		lastState:  make(map[ProductType]*feed.AgrTickMsg),
	}
}

// Run registers/deregisters websocket clients.
// Listens for tick messages from aggregator.
// Calculates and updates ticker string.
// Pushes to websocket updated ticker string.
func (s *Server) Run() {
	defer func() {
		close(s.wait)
	}()

	for {
		select {
		case client := <-s.register:
			s.clients[client] = struct{}{}
			//s.aggregator.ForceTick <- ProductBtcUsd
			//s.aggregator.ForceTick <- ProductEurUsd
			if s.cachedTick != "" {
				if !s.pushTickerToClient(client, s.cachedTick) {
					close(client.send)
					delete(s.clients, client)
				}
			}
		case client := <-s.deregister:
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				close(client.send)
			}
		case message, ok := <-s.aggregator.Tick:
			if !ok {
				for client := range s.clients {
					close(client.send)
				}
				return
			}
			if s.updateLastState(message) {
				for client := range s.clients {
					if !s.pushTickerToClient(client, s.cachedTick) {
						close(client.send)
						delete(s.clients, client)
					}
				}
			}
		}
	}
}

func (s *Server) Serve() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveWs(s, w, r)
	})
}

func (s *Server) Wait() {
	<-s.wait
}

func (s *Server) getProductState(product ProductType) (float64, int, int) {

	var price float64 = -1.0

	active, total := 0, 0
	if msg, ok := s.lastState[product]; ok {
		price = msg.BidPrice
		active = msg.ActiveSources
		total = msg.TotalSources
	}

	return price, active, total
}

func (s *Server) updateLastState(msg *feed.AgrTickMsg) bool {
	s.lastState[msg.Product] = msg
	if strTick := s.buildTickerString(); strTick != "" {
		if strTick != s.cachedTick {
			s.cachedTick = strTick
			return true
		}
	}
	return false
}

// Builds output ticker string in format:
// BTC/USD: 600   EUR/USD: 1.05   BTC/EUR: 550 Active sources: BTC/USD (3 of 3)  EUR/USD (2 of 3)
func (s *Server) buildTickerString() string {

	strTick := ""

	if priceBtcUsd, actBtcUsd, totBtcUsd := s.getProductState(ProductBtcUsd); priceBtcUsd > 0 {
		if priceEurUsd, actEurUsd, totEurUsd := s.getProductState(ProductEurUsd); priceEurUsd > 0 {
			// NOTE: Calulate BTC/EUR
			priceBtcEur := priceBtcUsd / priceEurUsd

			strPrice := fmt.Sprintf("%s: %.2f\t%s: %.2f\t%s: %.2f",
				ProductBtcUsd, priceBtcUsd,
				ProductEurUsd, priceEurUsd,
				ProductBtcEur, priceBtcEur,
			)
			strSrcBtc := fmt.Sprintf("%s (%d of %d)",
				ProductBtcUsd, actBtcUsd, totBtcUsd,
			)
			strSrcEur := fmt.Sprintf("%s (%d of %d)",
				ProductEurUsd, actEurUsd, totEurUsd,
			)
			strTick = fmt.Sprintf("%s Active sources: %s %s", strPrice, strSrcBtc, strSrcEur)
		}
	}

	return strTick
}

func (s *Server) pushTickerToClient(client *Client, strTick string) bool {
	select {
	case client.send <- []byte(strTick):
		return true
	default:
		return false
	}
}
