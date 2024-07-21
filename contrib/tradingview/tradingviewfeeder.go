package main

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	jsoniter "github.com/json-iterator/go"

	"github.com/alpacahq/marketstore/v4/executor"
	"github.com/alpacahq/marketstore/v4/planner"
	"github.com/alpacahq/marketstore/v4/plugins/bgworker"
	"github.com/alpacahq/marketstore/v4/utils"
	"github.com/alpacahq/marketstore/v4/utils/io"
	"github.com/alpacahq/marketstore/v4/utils/log"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const (
	chartsession        = "cs_denH2xgH"
	oneMinTimeframeStr  = "1Min"
	oneHourTimeframeStr = "1H"
	oneDayTimeframeStr  = "1D"
)

func resetBars(s *Bars) {
	*s = (*s)[:0]
}

func strip(text string) ([]Message, error) {
	noDataReg := regexp.MustCompile(`~m~\d+~m~~h~\d+`)
	if noDataReg.MatchString(text) {
		return []Message{}, nil
	}

	dataReg := regexp.MustCompile(`~m~\d+~m~`)
	dataParts := dataReg.Split(text, -1)

	var result []Message
	for _, part := range dataParts {
		if part != "" {
			var jsonData Message
			if err := json.Unmarshal([]byte(part), &jsonData); err != nil {
				return nil, err
			}
			result = append(result, jsonData)
		}
	}
	return result, nil
}

func unstrip(msg Message) (string, error) {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("~m~%d~m~%s", 2-8, string(jsonData)), nil
}

func convertPayloadToS1(msg Message) (S1, error) {
	// P[1]の"s1"データを取得
	s1Data, ok := msg.P[1].(map[string]interface{})["s1"]
	if !ok {
		return S1{}, fmt.Errorf("error extracting s1 data from message")
	}

	// s1データをJSONにエンコード
	s1Json, err := json.Marshal(s1Data)
	if err != nil {
		return S1{}, fmt.Errorf("error marshaling s1 data: %v", err)
	}

	// JSONをS1構造体にデコード
	var s1 S1
	if err := json.Unmarshal(s1Json, &s1); err != nil {
		return S1{}, fmt.Errorf("error decoding s1 JSON: %v", err)
	}

	return s1, nil
}

// FetcherConfig is a structure of tradingviewfeeder's parameters.
type FetcherConfig struct {
	Symbols       map[string][]string `json:"symbols"`
	BaseTimeframe string              `json:"base_timeframe"`
	Bars          int                 `json:"bars"`
}

// TradingViewFetcher is the main worker for TradingView.
type TradingViewFetcher struct {
	wg            *sync.WaitGroup
	config        map[string]interface{}
	url           string
	symbols       map[string][]string
	baseTimeframe *utils.Timeframe
}

// recast changes parsed JSON-encoded data represented as an interface to FetcherConfig structure.
func recast(config map[string]interface{}) (*FetcherConfig, error) {
	data, _ := json.Marshal(config)
	ret := FetcherConfig{}
	err := json.Unmarshal(data, &ret)
	if err != nil {
		return nil, fmt.Errorf("unmarshal FetcherConfig: %w", err)
	}

	return &ret, nil
}

func findLastTimestamp(tbk *io.TimeBucketKey) time.Time {
	cDir := executor.ThisInstance.CatalogDir
	query := planner.NewQuery(cDir)
	query.AddTargetKey(tbk)
	start := time.Unix(0, 0).In(utils.InstanceConfig.Timezone)
	end := time.Unix(math.MaxInt64, 0).In(utils.InstanceConfig.Timezone)
	query.SetRange(start, end)
	query.SetRowLimit(io.LAST, 1)
	parsed, err := query.Parse()
	if err != nil {
		log.Error(fmt.Sprintf("failed to parse query for %s", tbk))
		return time.Time{}
	}
	reader, err := executor.NewReader(parsed)
	if err != nil {
		log.Error(fmt.Sprintf("failed to create new reader for %s", tbk))
		return time.Time{}
	}
	csm, err := reader.Read()
	if err != nil {
		log.Error(fmt.Sprintf("failed to read query for %s", tbk))
		return time.Time{}
	}
	cs := csm[*tbk]
	if cs == nil || cs.Len() == 0 {
		return time.Time{}
	}
	ts, err := cs.GetTime()
	if err != nil {
		log.Error(fmt.Sprintf("failed to get time from query(tbk=%s)", tbk))
		return time.Time{}
	}
	return ts[0]
}

// NewBgWorker registers a new background worker.
func NewBgWorker(conf map[string]interface{}) (bgworker.BgWorker, error) {
	config, err := recast(conf)
	if err != nil {
		return nil, err
	}

	timeframeStr := oneMinTimeframeStr

	if config.BaseTimeframe != "" {
		timeframeStr = config.BaseTimeframe
	}

	// WaitGroupを使ってすべてのgoroutineの終了を待つ
	var wg sync.WaitGroup

	return &TradingViewFetcher{
		wg:            &wg,
		config:        conf,
		url:           "wss://data.tradingview.com/socket.io/websocket?from=",
		symbols:       config.Symbols,
		baseTimeframe: utils.NewTimeframe(timeframeStr),
	}, nil
}

func convertToCSM(tbk *io.TimeBucketKey, data OhlcvData) (csm io.ColumnSeriesMap, lastTime time.Time) {
	epoch := make([]int64, 0)
	open := make([]float64, 0)
	high := make([]float64, 0)
	low := make([]float64, 0)
	clos := make([]float64, 0)
	volume := make([]float64, 0)
	tradeCount := make([]int64, 0)

	parsedTime := time.Unix(data.Timestamp, 0)
	if parsedTime.After(lastTime) {
		lastTime = parsedTime
	}
	epoch = append(epoch, parsedTime.Unix())
	open = append(open, data.Open)
	high = append(high, data.High)
	low = append(low, data.Low)
	clos = append(clos, data.Close)
	volume = append(volume, data.Volume)
	tradeCount = append(tradeCount, data.TotalTrades)

	cs := io.NewColumnSeries()
	cs.AddColumn("Epoch", epoch)
	cs.AddColumn("Open", open)
	cs.AddColumn("High", high)
	cs.AddColumn("Low", low)
	cs.AddColumn("Close", clos)
	cs.AddColumn("Volume", volume)
	cs.AddColumn("Trades", tradeCount)
	csm = io.NewColumnSeriesMap()
	csm.AddColumnSeries(*tbk, cs)
	return csm, lastTime
}

func (cf *TradingViewFetcher) CreateWsUri(e, symbol string) string {
	return cf.url + url.QueryEscape("symbols/"+e+":"+symbol+"/")
}

func (cf *TradingViewFetcher) Reconnect(client *websocket.Conn, e, symbol string) *websocket.Conn {
	log.Info("reconnecting...")
	client.Close()
	var err error
	for {
		client, _, err = websocket.DefaultDialer.Dial(cf.CreateWsUri(e, symbol), nil)
		if err != nil {
			log.Fatal("reconnect.dial: %v", err)
			time.Sleep(10 * time.Second)
			continue
		}
		log.Info("reconnected")
		return client
	}
}

func (cf *TradingViewFetcher) Subscribe(client *websocket.Conn, e, symbol string) {
	instrument := e + ":" + symbol

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	timeframe := cf.baseTimeframe.String
	if strings.HasSuffix(timeframe, "Min") {
		timeframe = strings.Replace(timeframe, "Min", "", 1) // 1 = 1 minute
	}

	initMessages := InitMessages{
		Message{M: "set_data_quality", P: []interface{}{"low"}},
		Message{M: "set_auth_token", P: []interface{}{"unauthorized_user_token"}},
		Message{M: "chart_create_session", P: []interface{}{chartsession, ""}},
		Message{M: "resolve_symbol", P: []interface{}{chartsession, "symbol_1", "={\"symbol\":\"" + instrument + "\", \"adjustment\":\"splits\",\"session\":\"extended\"}"}},
		Message{M: "create_series", P: []interface{}{chartsession, "s1", "s1", "symbol_1", timeframe, 50}}, // 1 = 1 minute, 50 = 50 bars
	}

	for _, m := range initMessages {
		s, err := unstrip(m)
		if err != nil {
			log.Fatal("unstrip:", err)
		}
		err = client.WriteMessage(websocket.TextMessage, []byte(s))
		if err != nil {
			log.Fatal("message:", err)
		}
	}

	symbolDir := fmt.Sprintf("%s_%s", e, symbol)
	tbk := io.NewTimeBucketKey(symbolDir + "/" + cf.baseTimeframe.String + "/OHLCV")
	lastTimestamp := findLastTimestamp(tbk)
	log.Info("lastTimestamp for %s = %v", symbolDir, lastTimestamp)

	defer cf.wg.Done()
	defer client.Close()

	var bars Bars

	for {
		_, message, err := client.ReadMessage()
		if err != nil {
			log.Error("read: %v", err)
			err := client.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Error("write close (%v): %v", err)
			}
			client = cf.Reconnect(client, e, symbol)
			continue
		}
		payloads, err := strip(string(message))
		if err != nil {
			log.Error("strip: %v %v", err, message)
			return
		}

		if len(payloads) == 0 {
			err = client.WriteMessage(websocket.TextMessage, message)
			// log.Debug("keep-alive")
			if err != nil {
				log.Error("write: %v", err)
				return
			}
		}
		for _, payload := range payloads {
			if payload.M == "timescale_update" {
				// log.Debug("timescale_update")

				if len(bars) > 0 {
					var csm io.ColumnSeriesMap

					ohlcv := createOhlcvData(bars)
					// log.Debug("%v@%v %v", symbol, e, ohlcv)
					resetBars(&bars)
					csm, lastTime := convertToCSM(tbk, ohlcv)
					err = executor.WriteCSM(csm, false)
					if err != nil {
						log.Error("failed to write CSM for " + e + "_" + symbol + " data. err=" + err.Error())
					}

					// next fetch start point
					timeStart := lastTime.Add(cf.baseTimeframe.Duration)
					// for the next bar to complete, add it once more
					nextExpected := timeStart.Add(cf.baseTimeframe.Duration)
					now := time.Now()
					remaining := nextExpected.Sub(now)
					log.Debug("%s@%s %s %s left", symbol, e, lastTime, remaining)

				}

			} else if payload.M == "du" {
				s1, err := convertPayloadToS1(payload)
				if err != nil {
					log.Error("Error convertPayloadToS1 s1 data: %v", err)
					return
				}
				bars = append(bars, s1.S...)
				log.Debug("%v", s1.S)
			}
		}
	}
}

// Run grabs data in intervals from starting time to ending time.
// If query_end is not set, it will run forever.
func (cf *TradingViewFetcher) Run() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	numGoroutines := 0
	for e, v := range cf.symbols {
		for _, symbol := range v {
			numGoroutines += 1
			client, _, err := websocket.DefaultDialer.Dial(cf.CreateWsUri(e, symbol), http.Header{
				"Origin": []string{"https://www.tradingview.com"},
			})
			if err != nil {
				log.Fatal("dial:", err)
			}

			go cf.Subscribe(client, e, symbol)
		}
	}
	cf.wg.Add(numGoroutines)
	cf.wg.Wait()
}

func main() {
	instrument := "BYBIT:ETHUSDT"
	wsparam := url.QueryEscape("symbols/" + instrument + "/")
	// chartsession := "cs_Dj1BV8ochLL0"

	wsUri := "wss://data.tradingview.com/socket.io/websocket?from=" + wsparam
	// fmt.Println(wsUri)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	c, _, err := websocket.DefaultDialer.Dial(wsUri, http.Header{
		"Origin": []string{"https://www.tradingview.com"},
	})
	if err != nil {
		log.Fatal("dial:", err)
	}

	initMessages := InitMessages{
		Message{M: "set_data_quality", P: []interface{}{"low"}},
		Message{M: "set_auth_token", P: []interface{}{"unauthorized_user_token"}},
		Message{M: "chart_create_session", P: []interface{}{chartsession, ""}},
		Message{M: "resolve_symbol", P: []interface{}{chartsession, "symbol_1", "={\"symbol\":\"" + instrument + "\", \"adjustment\":\"splits\",\"session\":\"extended\"}"}},
		Message{M: "create_series", P: []interface{}{chartsession, "s1", "s1", "symbol_1", "1", 50}}, // 1 = 1 minute, 50 = 50 bars
	}

	for _, m := range initMessages {
		s, err := unstrip(m)
		if err != nil {
			log.Fatal("unstrip:", err)
		}
		// fmt.Printf("send: %s\n", s)
		err = c.WriteMessage(websocket.TextMessage, []byte(s))
		if err != nil {
			log.Fatal("message:", err)
		}
	}

	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		var bars Bars
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				fmt.Println("read:", err)
				return
			}

			// fmt.Printf("recv: %s\n", message)

			payloads, err := strip(string(message))
			if err != nil {
				fmt.Println("strip:", err)
				return
			}

			if len(payloads) == 0 {
				err = c.WriteMessage(websocket.TextMessage, message)
				fmt.Println("keep-alive")
				if err != nil {
					fmt.Println("write:", err)
					return
				}
			}
			for _, payload := range payloads {
				if payload.M == "timescale_update" {
					fmt.Println("timescale_update")
					if len(bars) > 0 {
						ohlcv := createOhlcvData(bars)
						fmt.Println(ohlcv)
						resetBars(&bars)
						// convertToCSM(tbk, bars)
					}
				} else if payload.M == "du" {
					s1, err := convertPayloadToS1(payload)
					if err != nil {
						fmt.Println("Error convertMsgToS1 s1 data:", err)
						return
					}
					bars = append(bars, s1.S...)
					// fmt.Printf("%v\n", s1.S)
				}
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			fmt.Println("interrupt")

			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				fmt.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}
