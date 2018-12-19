package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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
var syncIntervalMins int
var conf Config
var resolutions []string
var resolutionMins []int
var symbols []Symbol

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
	resolutions = conf.ChartConfig.Resolutions
	if len(resolutions) == 0 {
		// Set defaults
		resolutions = []string{"30", "60", "360", "1D"}
		conf.ChartConfig.Resolutions = resolutions
	}
	for i := 0; i < len(resolutions); i++ {
		multiplier := 0
		minStr := resolutions[i]
		if arr := strings.Split(resolutions[i], "D"); len(arr) > 1 {
			// Daily resolutions
			multiplier = 1440
			minStr = arr[0]
		} else if arr := strings.Split(resolutions[i], "W"); len(arr) > 1 {
			// Weekly resolutions
			multiplier = 10080
			minStr = arr[0]
		} else if arr := strings.Split(resolutions[i], "M"); len(arr) > 1 {
			// Monthly resolutions
			multiplier = 43200
			minStr = arr[0]
		}
		minutesInt, err := strconv.Atoi(minStr)
		if err == nil {
			resolutionMins = append(resolutionMins, minutesInt*multiplier)
		}
	}

	// Supported ticker symbols
	symbols = []Symbol{
		newSymbol(
			"Halo",
			"HALO",
			"Halo Platform",
			"0x0000000000000000000000000000000000000000",
			"0xd314d564c36c1b9fbbf6b440122f84da9a551029",
		),
		newSymbol(
			"VET",
			"VET",
			"Vechain",
			"0x280750ccb7554faec2079e8d8719515d6decdc84",
			"0xd314d564c36c1b9fbbf6b440122f84da9a551029",
		),
		newSymbol(
			"VTHO",
			"VTHO",
			"Vechain Thor",
			"0x0343350a2b298370381cac03fe3c525c28600b21",
			"0xd314d564c36c1b9fbbf6b440122f84da9a551029",
		),
		newSymbol(
			"DBET",
			"DBET",
			"DecentBet",
			"0x59195ebd987bde65258547041e1baed5fbd18e8b",
			"0xd314d564c36c1b9fbbf6b440122f84da9a551029",
		),
	}
	// Register http handlers
	registerHanders(map[string]func(http.ResponseWriter, *http.Request){
		// TradingView chart configuration data
		"/config": func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, conf.ChartConfig, ok200)
		},
		"/symbol_info": respondNotImplemented, // NOT REQUIRED
		"/symbols":     symbolsHandler,
		"/search":      searchHandler,
		"/history":     historyHandler, // TODO:
	})

	args := os.Args[1:]
	port := "3000"
	if len(args) > 0 {
		port = args[0]
	}
	go syncTrades()
	go syncTradesInterval()
	log.Println("HaloDEX chart data feed server started at port ", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func syncTradesInterval() {
	// Execute on interval
	for range time.Tick(time.Minute * time.Duration(conf.SyncIntervalMins)) {
		go syncTrades()
	}
}
func syncTrades() {
	sync("halo", true)
	sync("dbet", true)
	sync("vet", true)
	sync("vtho", true)
}

func registerHanders(handlers map[string]func(http.ResponseWriter, *http.Request)) {
	for path, handlerFunc := range handlers {
		http.HandleFunc(path, allowCORS(handlerFunc))
	}
}

func allowCORS(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[request] [config] %s", r.URL.Path)
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
