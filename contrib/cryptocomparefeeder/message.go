package main

type SubscribeAllComplete3 struct {
	Type    string `json:"TYPE"`
	Message string `json:"MESSAGE"`
	Info    string `json:"INFO"`
}

type SubscribeWelcome20 struct {
	Type                     string `json:"TYPE"`
	Message                  string `json:"MESSAGE"`
	ServerUptimeSeconds      int64  `json:"SERVER_UPTIME_SECONDS"`
	ServerName               string `json:"SERVER_NAME"`
	ServerTimeMs             int64  `json:"SERVER_TIME_MS"`
	ClientId                 int64  `json:"CLIENT_ID"`
	DataFormat               string `json:"DATA_FORMAT"`
	SocketId                 string `json:"SOCKET_ID"`
	SocketsActive            int64  `json:"SOCKETS_ACTIVE"`
	SocketsRemaining         int64  `json:"SOCKETS_REMAINING"`
	RatelimitMaxSecond       int64  `json:"RATELIMIT_MAX_SECOND"`
	RatelimitMaxMinute       int64  `json:"RATELIMIT_MAX_MINUTE"`
	RatelimitMaxHour         int64  `json:"RATELIMIT_MAX_HOUR"`
	RatelimitMaxDay          int64  `json:"RATELIMIT_MAX_DAY"`
	RatelimitMaxMonth        int64  `json:"RATELIMIT_MAX_MONTH"`
	RatelimitRemainingSecond int64  `json:"RATELIMIT_REMAINING_SECOND"`
	RatelimitRemainingMinute int64  `json:"RATELIMIT_REMAINING_MINUTE"`
	RatelimiteRemainingHour  int64  `json:"RATELIMIT_REMAINING_HOUR"`
	RatelimitRemainingDay    int64  `json:"RATELIMIT_REMAINING_DAY"`
	RatelimitRemainingMonth  int64  `json:"RATELIMIT_REMAINING_MONTH"`
}

type SubscribeComplete16 struct {
	Type    string `json:"TYPE"`
	Message string `json:"MESSAGE"`
	Sub     string `json:"SUB"`
}

type WarningMessage500 struct {
	Type      string `json:"TYPE"`
	Message   string `json:"MESSAGE"`
	Parameter string `json:"PARAMETER"`
	Info      string `json:"INFO"`
}

type HeatBeat999 struct {
	Type    string `json:"TYPE"`
	Message string `json:"MESSAGE"`
	Timems  int64  `json:"TIMEMS"`
}

type OhlcvData23 struct {
	Type        string  `json:"TYPE"`
	Market      string  `json:"MARKET"`
	FromSymbol  string  `json:"FROMSYMBOL"`
	ToSymbol    string  `json:"TOSYMBOL"`
	Ts          int64   `json:"TS"`
	Unit        string  `json:"UNIT"`
	Action      string  `json:"ACTION"`
	Open        float64 `json:"OPEN"`
	High        float64 `json:"HIGH"`
	Low         float64 `json:"LOW"`
	Close       float64 `json:"CLOSE"`
	VolumeFrom  float64 `json:"VOLUMEFROM"`
	VolumeTo    float64 `json:"VOLUMETO"`
	TotalTrades int64   `json:"TOTALTRADES"`
	FirstTs     int64   `json:"FIRSTTS"`
	LastTs      int64   `json:"LASTTS"`
	FirstPrice  float64 `json:"FIRSTPRICE"`
	MaxPrice    float64 `json:"MAXPRICE"`
	MinPrice    float64 `json:"MINPRICE"`
	LastPrice   float64 `json:"LASTPRICE"`
}
