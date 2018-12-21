package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/alien45/halo-info-bot/client"
)

// Http status codes
const ok200 = http.StatusOK
const created201 = http.StatusCreated
const err400 = http.StatusBadRequest
const err404 = http.StatusNotFound
const err500 = http.StatusInternalServerError
const err501 = http.StatusNotImplemented
const configFile = "./config.json"
const dataRootDir = "./data"

var err error
var dex client.DEX
var syncIntervalMins int // Sync trades every x minutes
var conf Config
var resolutions []string // sypported bar resolutions
var resolutionMins []int // bar resolution in minutes. Used when generating bars
var symbols []Symbol     // Supported symbols

// Config describes cofigurations and settings
type Config struct {
	HaloDEX            client.DEX  `json:"halodex"`
	SyncIntervalMins   int         `json:"syncintervalmins"`
	ChartConfig        ChartConfig `json:"chartconfig"`
	SplitTicker        string      `json:"splitticker"`
	PreSplitTime       time.Time   `json:"presplittime"`
	SplitAmount        float64     `json:"splitamount"`
	IgnoreTradesBefore time.Time   `json:"ignoretradesbefore"`
}

// ChartConfig ...
type ChartConfig struct {
	Resolutions    []string `json:"supported_resolutions"`
	GroupRequest   bool     `json:"supports_group_request"`
	Marks          bool     `json:"supports_marks"`
	Search         bool     `json:"supports_search"`
	TimescaleMarks bool     `json:"supports_timescale_marks"`
}

func main() {
	jsonStr, err := client.ReadFile(configFile)
	panicIf(err, "Failed to read config file: "+configFile)
	err = json.Unmarshal([]byte(jsonStr), &conf)
	panicIf(err, "Failed to unmarshal config json")
	dex = conf.HaloDEX
	syncIntervalMins = conf.SyncIntervalMins
	setupResolutions()
	// Update supported tickers/symbols
	updateSymbols()
	// Register http handlers
	registerHanders(map[string]func(http.ResponseWriter, *http.Request){
		// TradingView chart configuration data
		"/config": func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, conf.ChartConfig, ok200)
		},
		"/symbol_info": respondNotImplemented,
		"/symbols":     symbolsHandler,
		"/search":      searchHandler,
		"/history":     historyHandler,
	})

	args := os.Args[1:]
	port := "3000"
	if len(args) > 0 {
		port = args[0]
	}
	go syncTradesInterval(true)
	log.Println("HaloDEX chart data feed server started at port ", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func syncTradesInterval(execOnInit bool) {
	if execOnInit {
		syncTrades()
	}
	// Execute on interval
	for range time.Tick(time.Minute * time.Duration(conf.SyncIntervalMins)) {
		go syncTrades()
	}
}

func syncTrades() {
	for _, symbol := range symbols {
		sync(symbol.Ticker, true)
	}
}

func registerHanders(handlers map[string]func(http.ResponseWriter, *http.Request)) {
	for path, handlerFunc := range handlers {
		http.HandleFunc(path, allowCORS(handlerFunc))
	}
}

func allowCORS(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[request] %s | [ip] %s", r.URL.RequestURI(), r.RemoteAddr)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,access-control-allow-origin, access-control-allow-headers")
		handler(w, r)
	}
}

// symbolsHandler responds with "Not implemented - 501" as this feature is currently not planned
func respondNotImplemented(w http.ResponseWriter, r *http.Request) {
	log.Printf("[request] %s", r.URL.Path)
	respondError(w, "", err501)
}

func respondJSON(w http.ResponseWriter, content interface{}, statusCode int) {
	b, err := json.Marshal(content)
	if respondIfError(err, w, "Something went wrong!", err500) {
		return
	}
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(b)
	if err != nil {
		log.Println("[response] [error]", err)
		return
	}
	log.Printf("[response] [status%d] %s\n", statusCode, http.StatusText(statusCode))
}

func respondIfError(err error, w http.ResponseWriter, msg string, statusCode int) bool {
	if err == nil {
		return false
	}
	respondError(w, msg, statusCode)
	return true
}

func respondError(w http.ResponseWriter, msg string, statusCode int) {
	if statusCode == 0 {
		statusCode = err400
	}
	if msg == "" {
		msg = http.StatusText(statusCode)
	}
	http.Error(w, msg, statusCode)
	log.Printf("[response] [status%d] %s\n", statusCode, msg)
}

func panicIf(err error, msg string) {
	if err != nil {
		fmt.Printf("%s: %+v\n", msg, err)
		panic(err)
	}
}
