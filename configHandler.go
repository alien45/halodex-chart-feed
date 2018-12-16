package main

import (
	"net/http"
)

var supportedResolutions = []string{
	// "1",
	// "5",
	// "15",
	"30",
	"60",
	"360",
	"720",
	// "1D",
	//	"1W",
	//	"1M",
}

// Supported resolutions in minutes
var supportedResolutionsInt = []int{
	// 1,
	// 5,
	// 15,
	30,
	60,
	360,
	720,
}

// configHandler returns TradingView chart data feed configuration data
func configHandler(w http.ResponseWriter, r *http.Request) {
	result := map[string]interface{}{
		"supported_resolutions":    supportedResolutions,
		"supports_group_request":   false,
		"supports_marks":           false,
		"supports_search":          true,
		"supports_timescale_marks": false,
	}
	respondJSON(w, result, ok200)
}
