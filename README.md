# BTCTicker

BTCTicker is a Bitcoin exchange rates aggregtor.

Currently supported sources:
* [GDAX] (https://www.gdax.com/) (BTC/USD)
* [BTCe] (https://btc-e.com) (BTC/USD)
* [Fixer.io] (http://fixer.io/) (EUR/USD)
* [rate-exchange-1.appspot.com] (http://rate-exchange-1.appspot.com/) (EUR/USD)

### Implementation
- client/server mode using [Websocket] (http://github.com/gorilla/websocket)

### Installation
`go get github.com/korobool/btcticker`

### Launch server
`go run main.go -server [-addr <address:port>]`

### Attach viewer
`go run main.go [-addr <address:port>]`
