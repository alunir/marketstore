package main

import "fmt"

type Bar struct {
	Timestamp int64
	OHLCV     [5]float64
}

func (v *Bar) UnmarshalJSON(data []byte) error {
	var temp []interface{}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	if len(temp) != 6 {
		return fmt.Errorf("expected 6 elements, got %d", len(temp))
	}

	// 最初の要素をint64に変換
	first, ok := temp[0].(float64)
	if !ok {
		return fmt.Errorf("expected float64 for the first element, got %T", temp[0])
	}
	v.Timestamp = int64(first)

	// 残りの要素をfloat64に変換
	for i := 1; i < 6; i++ {
		rest, ok := temp[i].(float64)
		if !ok {
			return fmt.Errorf("expected float64 for element %d, got %T", i, temp[i])
		}
		v.OHLCV[i-1] = rest
	}

	return nil
}

type BarIndex struct {
	I int `json:"i"`
	V Bar `json:"v"`
}

type OhlcvData struct {
	Timestamp   int64   `json:"timestamp"`
	Open        float64 `json:"open"`
	High        float64 `json:"high"`
	Low         float64 `json:"low"`
	Close       float64 `json:"close"`
	Volume      float64 `json:"volume"`
	TotalTrades int64   `json:"number"`
}

func createOhlcvData(bars Bars) OhlcvData {
	l := len(bars)
	ohlcvData := OhlcvData{
		Timestamp:   bars[l-1].V.Timestamp,
		Open:        bars[l-1].V.OHLCV[0],
		High:        bars[l-1].V.OHLCV[1],
		Low:         bars[l-1].V.OHLCV[2],
		Close:       bars[l-1].V.OHLCV[3],
		Volume:      bars[l-1].V.OHLCV[4],
		TotalTrades: int64(len(bars)),
	}
	return ohlcvData

}

type NS struct {
	D       string `json:"d"`
	Indexes string `json:"indexes"`
}

type LBS struct {
	BarCloseTime int64 `json:"bar_close_time"`
}

type Bars []BarIndex

type S1 struct {
	S   Bars   `json:"s"`
	NS  NS     `json:"ns"`
	T   string `json:"t"`
	LBS LBS    `json:"lbs"`
}

type Message struct {
	M string        `json:"m"`
	P []interface{} `json:"p"`
}

type InitMessages []Message
