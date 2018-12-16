package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/alien45/halo-info-bot/client"
)

const historyStatusNoData = "no_data"
const historyStatusOk = "ok"
const historyStatusError = "error"

var resolutionCache map[string][]Bar
var cachedBars map[string]map[string][]Bar // Symbol : resolution : []Bar

// History as described here: https://github.com/tradingview/charting_library/wiki/UDF#bars
type History struct {
	// Valid statuses : ok | error | no_data
	Status string `json:"s"`
	// only when Status == error
	ErrorMessage string `json:"errmsg"`
	// Unix Epoch time in seconds
	BarTime      []int64   `json:"t"`
	ClosingPrice []float64 `json:"c"`
	OpeningPrice []float64 `json:"o"`
	HighPrice    []float64 `json:"h"`
	LowPrice     []float64 `json:"l"`
	Volume       []float64 `json:"v"`
	// Unix Epoch time of the next bar.
	// Only if status == no_data
	NextTime int64 `json:"nextTime"`
}

// Bar as described here: https://github.com/tradingview/charting_library/wiki/UDF#bars
// type Bar struct {
// 	// Unix Epoch time in seconds
// 	Time         time.Time
// 	UnixTime     int64   `json:"t"`
// 	ClosingPrice float64 `json:"c"`
// 	OpeningPrice float64 `json:"o"`
// 	HighPrice    float64 `json:"h"`
// 	LowPrice     float64 `json:"l"`
// 	Volume       float64 `json:"v"`
// 	Prices       []float64
// }

func historyHandler(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	symbol := strings.ToLower(params["symbol"][0])
	resolution := params["resolution"][0]
	log.Println("Resolution:", resolution)
	from, _ := strconv.ParseInt(params["from"][0], 0, 64)
	to, _ := strconv.ParseInt(params["to"][0], 0, 64)
	h := History{}
	h.Status = historyStatusOk
	nextTime := int64(0)
	bars, err := getResolution(symbol, resolution)
	if respondIfError(err, w, "Failed to read file or symbol not found", err500) {
		return
	}
	for i := 0; i < len(bars); i++ {
		t := bars[i].UnixTime
		if t >= from && t <= to {
			h.BarTime = append(h.BarTime, t)
			h.ClosingPrice = append(h.ClosingPrice, bars[i].ClosingPrice)
			h.OpeningPrice = append(h.OpeningPrice, bars[i].OpeningPrice)
			h.HighPrice = append(h.HighPrice, bars[i].HighPrice)
			h.LowPrice = append(h.LowPrice, bars[i].LowPrice)
			h.Volume = append(h.Volume, bars[i].Volume)
		}
		// if t > to {
		// 	// end of requested time range
		// 	if i < len(bars)-1 {
		// 		// nextTime = t
		// 	}
		// 	break
		// }
	}

	if len(h.BarTime) == 0 {
		h.Status = historyStatusNoData
		h.NextTime = nextTime
	}
	respondJSON(w, h, ok200)
}

func getResolution(symbol, resolution string) (bars []Bar, err error) {
	if cachedBars == nil {
		cachedBars = map[string]map[string][]Bar{}
	}
	if cachedBars[symbol] == nil {
		cachedBars[symbol] = map[string][]Bar{}
	}
	if bars, exists := cachedBars[symbol][resolution]; exists {
		return bars, nil
	}
	log.Printf("Loading bars from storage: %s/%s", symbol, resolution)
	filename := fmt.Sprintf("%s/%s/%s.json", dataRootDir, symbol, resolution)
	jsonStr, err := client.ReadFile(filename)
	if err != nil {
		return
	}
	err = json.Unmarshal([]byte(jsonStr), &bars)
	cachedBars[symbol][resolution] = bars
	return
}
