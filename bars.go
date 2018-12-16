package main

import (
	"fmt"
	"time"

	"github.com/alien45/halo-info-bot/client"
)

// Bar as described here: https://github.com/tradingview/charting_library/wiki/UDF#bars
type Bar struct {
	// Unix Epoch time in seconds
	Time         time.Time
	TimeEnd      time.Time
	UnixTime     int64   `json:"t"`
	ClosingPrice float64 `json:"c"`
	OpeningPrice float64 `json:"o"`
	HighPrice    float64 `json:"h"`
	LowPrice     float64 `json:"l"`
	Volume       float64 `json:"v"`
	Prices       []float64
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
	//bar.Prices = append(bar.Prices, price)
}

func generateNSaveResolution(trades []client.Trade, res int, parentDir string) (bars []Bar, err error) {
	// Generate 60 minute bars
	bars, err = generateResolution(trades, res)
	if err != nil {
		return nil, err
	}
	return bars, client.SaveJSONFile(fmt.Sprintf("%s/%d.json", parentDir, res), bars)
}

// Expects trades to be in decending order
func generateResolution(trades []client.Trade, resolutionMins int) (bars []Bar, err error) {
	bar := Bar{}
	// Ignore the first few TEST trades by Halo team
	startDate, _ := time.Parse(time.RFC3339, "2018-10-20T00:00:00Z")
	for i := len(trades) - 1; i >= 0; i-- {
		t := trades[i]
		if t.Time.Before(startDate) {
			continue
		}
		if bar.Time.IsZero() {
			// Find closest starting point
			bar.Time = t.Time.Truncate(time.Minute * time.Duration(resolutionMins))
			bar.TimeEnd = bar.Time.Add(time.Minute * time.Duration(resolutionMins))
			// log.Println(t.Time, bar.Time, bar.TimeEnd)
			bar.UnixTime = bar.Time.Unix()
			bar.SetPrices(t.Price)
			continue
		}
		// bar time is set. check if current trade is within the bar time range
		// log.Println(t.Time.UTC(), bar.TimeEnd, " | After: ", t.Time.After(bar.TimeEnd))
		if t.Time.After(bar.TimeEnd) {
			bars = append(bars, bar)
			// start of the next bar
			bar = Bar{}
		}
		bar.SetPrices(t.Price)
	}
	return
}