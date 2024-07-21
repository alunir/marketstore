package main

import (
	"testing"

	"github.com/alpacahq/marketstore/v4/utils/io"
)

// Test convertToCSM function
func TestConvertToCSM(t *testing.T) {
	// Create a TimeBucketKey
	tbk := io.NewTimeBucketKey("AAPL/1Min/OHLCV")

	rate := OhlcvData{
		Timestamp:   1721565180,
		Open:        300,
		High:        400,
		Low:         200,
		Close:       350,
		Volume:      36.50037,
		TotalTrades: 46,
	}
	// Call convertToCSM
	csm, lastTime := convertToCSM(tbk, rate)
	// Check the result
	if csm == nil {
		t.Errorf("csm is nil")
	}
	if lastTime.IsZero() {
		t.Errorf("lastTime is zero")
	}
}
