package main

import (
	//"bufio"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/korobool/btcticker/feed"
	"github.com/korobool/btcticker/server"
	"github.com/korobool/btcticker/viewer"
)

func runViewer(addr string, interrupt chan struct{}) {

	log.SetFlags(log.Ldate)
	log.SetOutput(os.Stderr)

	//viewer.WsConnect(addr, bufio.NewWriter(os.Stdout), interrupt)
	viewer.WsConnect(addr, os.Stdout, interrupt)
}

func runServer(addr string, interrupt chan struct{}) {

	aggregator, err := feed.NewAggregator(interrupt)
	if err != nil {
		log.Fatalf("failed: %s", err)
	}
	go aggregator.Run([]string{"gdax", "btce", "fixe", "fake_eurusd"})

	server := server.New(aggregator)
	go server.Run()

	server.Serve()

	go func() {
		<-interrupt
		aggregator.Wait()
		server.Wait()
		os.Exit(1)
	}()

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("ListenAndServe: %v", err)
	}
}

func main() {
	serverMode := flag.Bool("server", false, "run in server mode")
	addr := flag.String("addr", "localhost:29999", "address to connect/bind")

	flag.Parse()

	sysInterrupt := make(chan os.Signal, 1)
	signal.Notify(sysInterrupt, os.Interrupt)
	interrupt := make(chan struct{})

	go func() {
		<-sysInterrupt
		close(interrupt)
	}()

	if *serverMode {
		runServer(*addr, interrupt)
	} else {
		runViewer(*addr, interrupt)
	}
}
