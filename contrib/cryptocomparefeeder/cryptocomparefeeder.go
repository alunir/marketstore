package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	json "github.com/json-iterator/go"

	"github.com/alpacahq/marketstore/v4/executor"
	"github.com/alpacahq/marketstore/v4/planner"
	"github.com/alpacahq/marketstore/v4/plugins/bgworker"
	"github.com/alpacahq/marketstore/v4/utils"
	"github.com/alpacahq/marketstore/v4/utils/io"
	"github.com/alpacahq/marketstore/v4/utils/log"
)

const (
	defaultHTTPTimeout  = 10 * time.Second
	oneMinTimeframeStr  = "1Min"
	oneHourTimeframeStr = "1H"
	oneDayTimeframeStr  = "1D"
)

type JSONParams struct {
	Action string   `json:"action"`
	Subs   []string `json:"subs"`
}

// FetcherConfig is a structure of binancefeeder's parameters.
type FetcherConfig struct {
	Symbols       map[string][]string `json:"symbols"`
	BaseTimeframe string              `json:"base_timeframe"`
}

// CryptocompareFetcher is the main worker for Cryptocompare.
type CryptocompareFetcher struct {
	config         map[string]interface{}
	url            string
	subadd         []byte
	client         *websocket.Conn
	symbols        map[string][]string
	baseCurrencies []string
	queryStart     time.Time
	baseTimeframe  *utils.Timeframe
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

// Convert timeframe string to cryptocompare's timeframe string.
// 1Min -> m, 1H -> h, 1D -> d, otherwise error
func convertTimeframe(timeframeStr string) (string, error) {
	switch timeframeStr {
	case oneMinTimeframeStr:
		return "m", nil
	case oneHourTimeframeStr:
		return "h", nil
	case oneDayTimeframeStr:
		return "d", nil
	default:
		return "", fmt.Errorf("incorrect timeframe format: %s", timeframeStr)
	}
}

// findMinDuration returns the minimum duration from the given slice.
func findMinDuration(durations []time.Duration) time.Duration {
	min := durations[0]
	for _, duration := range durations {
		if duration < min {
			min = duration
		}
	}
	return min
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
	var queryStart time.Time
	timeframeStr := oneMinTimeframeStr
	baseCurrencies := []string{"USDT"}

	if config.BaseTimeframe != "" {
		timeframeStr = config.BaseTimeframe
	}

	subscriptionTimeframeStr, err := convertTimeframe(timeframeStr)
	if err != nil {
		log.Error("Incorrect timeframe format: %v", err)
		return nil, fmt.Errorf("incorrect timeframe format: %w", err)
	}

	// this is where you paste your api key
	apiKey := os.Getenv("CRYPTOCOMPARE_API_KEY")
	if apiKey == "" {
		log.Error("CRYPTOCOMPARE_API_KEY environment variable not set")
		return nil, fmt.Errorf("CRYPTOCOMPARE_API_KEY environment variable not set")
	}

	url := "wss://streamer.cryptocompare.com/v2?api_key=" + apiKey

	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal("dial:", err)
		return nil, fmt.Errorf("dial: %w", err)
	}

	subscriptions := make([]string, 0)
	for e, v := range config.Symbols {
		for _, symbol := range v {
			s := strings.Replace(symbol, "-", "~", 1)
			subscriptions = append(subscriptions, fmt.Sprintf("24~%s~%s~%s", e, s, subscriptionTimeframeStr))
		}
	}

	jsonObj := JSONParams{
		Action: "SubAdd",
		Subs:   subscriptions,
	}
	s, _ := json.Marshal(jsonObj)
	err = c.WriteMessage(websocket.TextMessage, s)
	if err != nil {
		log.Fatal("message:", err)
	}

	return &CryptocompareFetcher{
		config:         conf,
		url:            url,
		subadd:         s,
		client:         c,
		baseCurrencies: baseCurrencies,
		symbols:        config.Symbols,
		queryStart:     queryStart,
		baseTimeframe:  utils.NewTimeframe(timeframeStr),
	}, nil
}

func convertToCSM(tbk *io.TimeBucketKey, rate OhlcvData23) (csm io.ColumnSeriesMap, lastTime time.Time) {
	epoch := make([]int64, 0)
	open := make([]float64, 0)
	high := make([]float64, 0)
	low := make([]float64, 0)
	clos := make([]float64, 0)
	volume := make([]float64, 0)
	tradeCount := make([]int64, 0)

	parsedTime := time.Unix(rate.Ts, 0)
	if parsedTime.After(lastTime) {
		lastTime = parsedTime
	}
	epoch = append(epoch, parsedTime.Unix())
	open = append(open, rate.Open)
	high = append(high, rate.High)
	low = append(low, rate.Low)
	clos = append(clos, rate.Close)
	volume = append(volume, rate.VolumeTo)
	tradeCount = append(tradeCount, rate.TotalTrades)

	cs := io.NewColumnSeries()
	cs.AddColumn("Epoch", epoch)
	cs.AddColumn("Open", open)
	cs.AddColumn("High", high)
	cs.AddColumn("Low", low)
	cs.AddColumn("Close", clos)
	cs.AddColumn("Volume", volume)
	cs.AddColumn("Number", tradeCount)
	log.Debug("%s: between %s - %s", tbk.String(), rate.FirstTs, rate.LastTs)
	csm = io.NewColumnSeriesMap()
	csm.AddColumnSeries(*tbk, cs)
	return csm, lastTime
}

// Run grabs data in intervals from starting time to ending time.
// If query_end is not set, it will run forever.
func (cf *CryptocompareFetcher) Run() {
	errTs := time.Now()

	for e, v := range cf.symbols {
		for _, symbol := range v {
			symbolDir := fmt.Sprintf("%s_%s", e, symbol)
			tbk := io.NewTimeBucketKey(symbolDir + "/" + cf.baseTimeframe.String + "/OHLCV")
			lastTimestamp := findLastTimestamp(tbk)
			log.Info("lastTimestamp for %s = %v", symbolDir, lastTimestamp)
		}
	}

	for {
		_, message, err := cf.client.ReadMessage()
		if err != nil {
			if time.Now().Add(-25 * time.Minute).After(errTs) {
				panic("no data for 25 minutes")
			}
			log.Error("read:", err.Error())
			err := cf.client.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Error("write close:", err)
			}
			cf.client.Close()
			cf.client, _, err = websocket.DefaultDialer.Dial(cf.url, nil)
			if err != nil {
				log.Fatal("dial:", err)
				time.Sleep(time.Minute)
				continue
			}
			err = cf.client.WriteMessage(websocket.TextMessage, cf.subadd)
			if err != nil {
				log.Fatal("dial:", err)
				time.Sleep(time.Minute)
			}
			continue
		}
		errTs = time.Now()
		var resp OhlcvData23
		err = json.Unmarshal(message, &resp)
		if err != nil {
			log.Error("unmarshal:", err)
			continue
		}
		switch resp.Type {
		case "3":
			var m MessageInfo
			err = json.Unmarshal(message, &m)
			if err != nil {
				log.Error("unmarshal:", err)
				continue
			}
			log.Info("%s %s", m.Message, m.Info)
			time.Sleep(time.Second)
			continue
		case "16":
			var m SubscribeComplete16
			err = json.Unmarshal(message, &m)
			if err != nil {
				log.Error("unmarshal:", err)
				continue
			}
			log.Info("%s %s", m.Message, m.Sub)
			continue
		case "20":
			var m SubscribeWelcome20
			err = json.Unmarshal(message, &m)
			if err != nil {
				log.Error("unmarshal:", err)
				continue
			}
			log.Info("%s", m.Message)
			continue
		case "429":
			var m MessageInfo
			err = json.Unmarshal(message, &m)
			if err != nil {
				log.Error("unmarshal:", err)
				continue
			}
			log.Warn("%s %s", m.Message, m.Info)
			continue
		case "500":
			var m WarningMessage500
			err = json.Unmarshal(message, &m)
			if err != nil {
				log.Error("unmarshal:", err)
				continue
			}
			log.Info("message: %s %s %s", m.Message, m.Parameter, m.Info)
			continue
		case "24":
			var csm io.ColumnSeriesMap

			symbolDir := fmt.Sprintf("%s_%s-%s", resp.Market, resp.FromSymbol, resp.ToSymbol)
			tbk := io.NewTimeBucketKey(symbolDir + "/" + cf.baseTimeframe.String + "/OHLCV")

			csm, lastTime := convertToCSM(tbk, resp)
			err = executor.WriteCSM(csm, false)
			if err != nil {
				log.Error("failed to write CSM for " + resp.Market + " data. err=" + err.Error())
			}

			// next fetch start point
			timeStart := lastTime.Add(cf.baseTimeframe.Duration)
			// for the next bar to complete, add it once more
			nextExpected := timeStart.Add(cf.baseTimeframe.Duration)
			now := time.Now()
			remaining := nextExpected.Sub(now)
			log.Debug("%s-%s@%s (%s) %s %s left", resp.FromSymbol, resp.ToSymbol, resp.Market, resp.Action, lastTime, remaining)
		case "999":
			var m HeatBeat999
			err = json.Unmarshal(message, &m)
			if err != nil {
				log.Error("unmarshal:", err)
				continue
			}
			log.Debug("heatbeat: %d", m.Timems)
			continue
		default:
			log.Error("unknown message type: %s", string(message))
		}
	}
}

func main() {
	flag.Parse()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// this is where you paste your api key
	apiKey := os.Getenv("CRYPTOCOMPARE_API_KEY")
	if apiKey == "" {
		log.Error("CRYPTOCOMPARE_API_KEY environment variable not set")
		return
	}

	c, _, err := websocket.DefaultDialer.Dial("wss://streamer.cryptocompare.com/v2?api_key="+apiKey, nil)
	if err != nil {
		log.Fatal("dial:", err)
	}

	jsonObj := JSONParams{Action: "SubAdd", Subs: []string{"24~CCCAGG~BTC~USD~m"}}
	s, _ := json.Marshal(jsonObj)
	fmt.Println(string(s))
	err = c.WriteMessage(websocket.TextMessage, s)
	if err != nil {
		log.Fatal("message:", err)
	}

	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				fmt.Println("read:", err)
				return
			}
			fmt.Printf("recv: %s\n", message)
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
