package main

import (
	"log"
	"net/http"

	"github.com/demyan/btcticker/feed"
	"github.com/demyan/btcticker/server"
)

var addr = ":8080"

func main() {

	aggregator, err := feed.NewAggregator()
	if err != nil {
		log.Fatalf("failed: %s", err)
	}
	go aggregator.Run([]string{"gdax", "fake_eurusd"})

	server := server.New(aggregator)
	go server.Run()
	server.Serve()

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
