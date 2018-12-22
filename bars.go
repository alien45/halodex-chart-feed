package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/alien45/halo-info-bot/client"
)

// Bar as described here: https://github.com/tradingview/charting_library/wiki/UDF#bars
type Bar struct {
	Time         time.Time
	TimeEnd      time.Time
	UnixTime     int64   `json:"t"` // Unix Epoch time in seconds
	ClosingPrice float64 `json:"c"`
	OpeningPrice float64 `json:"o"`
	HighPrice    float64 `json:"h"`
	LowPrice     float64 `json:"l"`
	Volume       float64 `json:"v"`
}

// SetPrices ...
func (bar *Bar) SetPrices(price float64) {
	if bar.OpeningPrice == 0 {
		bar.OpeningPrice = price
	}
	if bar.HighPrice < price {
		bar.HighPrice = price
	}
	if bar.LowPrice == 0 {
		bar.LowPrice = price
	} else if bar.LowPrice > price {
		bar.LowPrice = price
	}
	bar.ClosingPrice = price
}

func setupResolutions() {
	resolutions = conf.ChartConfig.Resolutions
	if len(resolutions) == 0 {
		// Set defaults
		resolutions = []string{"30", "60", "360", "1D"}
		conf.ChartConfig.Resolutions = resolutions
	}
	for i := 0; i < len(resolutions); i++ {
		multiplier := 1
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
	log.Println("Supported resolutions: ", resolutions, "=> minutes: ", resolutionMins)
}

func generateNSaveBars(ticker, parentDir string, trades []client.Trade) {
	log.Println("Generating bars")
	if cachedBars == nil {
		cachedBars = map[string]map[string][]Bar{}
	}
	if cachedBars[ticker] == nil {
		cachedBars[ticker] = map[string][]Bar{}
	}
	// Check if there's any pre-split conversion required
	if strings.ToUpper(conf.SplitTicker) == strings.ToUpper(ticker) && conf.SplitAmount > 0 {
		for i, t := range trades {
			if t.Time.Before(conf.PreSplitTime) {
				// Convert trade amount and price before split to match post-split ratio
				trades[i].Amount *= conf.SplitAmount
				trades[i].Price /= conf.SplitAmount
			}
		}
	}
	// Generate resolution bars
	for i, resName := range resolutions {
		res := resolutionMins[i]
		log.Println("Generating resolution: ", resName, res)
		bars, err := generateNSaveResolution(trades, res, resName, parentDir)
		if err != nil {
			log.Printf("Failed to generate bar for %s resolution %s\n", ticker, resName)
			continue
		}
		// update cache
		cachedBars[ticker][fmt.Sprint(res)] = bars
	}
}

func generateNSaveResolution(trades []client.Trade, res int, resName, parentDir string) (bars []Bar, err error) {
	// Generate X minute resolution bars
	bars, err = generateResolution(trades, res)
	if err != nil {
		return nil, err
	}
	return bars, client.SaveJSONFile(fmt.Sprintf("%s/%s.json", parentDir, resName), bars)
}

// Expects trades to be in decending order
func generateResolution(trades []client.Trade, resolutionMins int) (bars []Bar, err error) {
	bar := Bar{}
	// Ignore the first few TEST trades by Halo team
	for i := len(trades) - 1; i >= 0; i-- {
		t := trades[i]
		if t.Time.Before(conf.IgnoreTradesBefore) {
			continue
		}

		if bar.Time.IsZero() {
			// Find closest starting point
			bar.Time = t.Time.Truncate(time.Minute * time.Duration(resolutionMins))
			bar.TimeEnd = bar.Time.Add(time.Minute * time.Duration(resolutionMins))
			bar.UnixTime = bar.Time.Unix()
			bar.SetPrices(t.Price)
			continue
		}
		// bar time is set. check if current trade is within the bar time range
		if t.Time.After(bar.TimeEnd) {
			bars = append(bars, bar)
			// start of the next bar
			bar = Bar{}
		}
		bar.SetPrices(t.Price)
		bar.Volume += t.Amount
	}
	return
}
