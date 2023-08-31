package main

import (
	"testing"

	"github.com/alpacahq/marketstore/v4/utils/io"
)

// Test convertToCSM function
func TestConvertToCSM(t *testing.T) {
	// Create a TimeBucketKey
	tbk := io.NewTimeBucketKey("AAPL/1Min/OHLCV")
	// Create a bitmex.TradeBucketedResponse
	rate := CryptoCompareOhlcvData{
		Ts:         1577836800000,
		Open:       300,
		High:       400,
		Low:        200,
		Close:      350,
		VolumeFrom: 1000,
		VolumeTo:   1000,
		FirstTs:    1577836800000,
		LastTs:     1577836800000,
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
