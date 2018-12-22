package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/alien45/halo-info-bot/client"
)

// Synchronizes trade history from HaloDEX to local directory
/*
	 Steps:
	 1. Load existing history file if exists.
	 2. Get the last trade's timestamp if file exists.
		 Otherwise use 0 to retrieve trades since inception.
	 3.	Add retrieved trades to loaded file and save
	 4. Re-generate bars
	 5. Update in-memory cached bars
*/
func sync(ticker string, generateBars bool) (err error) {
	ticker = strings.ToLower(ticker)
	log.Println("Syncing trades: ", ticker)
	dir := fmt.Sprintf("%s/%s", dataRootDir, ticker)
	tradesFile := dir + "/trades.json"

	txt, err := client.ReadFile(tradesFile)
	if err != nil {
		if _, err = os.Stat(tradesFile); !os.IsNotExist(err) {
			log.Println(err)
			return
		}
		// makes sure file path exists when saving file
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			log.Println("Failed to create directory", dir)
			return
		}
		txt = "[]"
	}
	trades := []client.Trade{}
	err = json.Unmarshal([]byte(txt), &trades)
	if err != nil {
		log.Println("Sync failed", err)
		return
	}
	log.Printf("Loaded existing trades: %d", len(trades))
	symbol := Symbol{}
	for i := 0; i < len(symbols); i++ {
		if strings.ToLower(symbols[i].Ticker) == strings.ToLower(ticker) {
			symbol = symbols[i]
		}
	}
	if symbol.Ticker == "" {
		return errors.New("Symbol not found")
	}

	startTime := time.Time{}
	if len(trades) > 0 {
		// add a nanosecond to make sure last item is not retrieved again
		startTime = trades[0].Time.UTC().Add(time.Nanosecond)
	}

	newTrades, err := dex.GetTradesByTime(symbol.Address, symbol.BaseAddress, startTime)
	if err != nil {
		log.Println("Failed to retrieve trades", err)
		return
	}
	trades = append(newTrades, trades...)
	err = client.SaveJSONFileLarge(tradesFile, trades)
	log.Println("File: ", tradesFile)
	if err != nil {
		log.Println("File save failed", tradesFile, err)
		return
	}

	log.Printf("Sync complete. Ticker: %s, Total Trades: %d, New: %d", ticker, len(trades), len(newTrades))
	if generateBars {
		generateNSaveBars(ticker, dir, trades)
	}
	return
}
