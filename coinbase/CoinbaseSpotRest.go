package coinbase

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	. "github.com/deforceHK/goghostex"
)

type Spot struct {
	*Coinbase
}

// public api
func (spot *Spot) GetTicker(pair Pair) (*Ticker, []byte, error) {
	t := struct {
		Volume float64 `json:"volume,string"`
		Buy    float64 `json:"bid,string"`
		Sell   float64 `json:"ask,string"`
	}{}

	s := struct {
		Last float64 `json:"last,string"`
		High float64 `json:"high,string"`
		Low  float64 `json:"low,string"`
	}{}

	wg := sync.WaitGroup{}
	wg.Add(2)

	var tickerResp []byte
	var tickerErr, statErr error
	go func() {
		defer wg.Done()
		uri := fmt.Sprintf("/products/%s/ticker", pair.ToSymbol("-", true))
		tickerResp, tickerErr = spot.DoRequest("GET", uri, "", &t)
	}()

	go func() {
		defer wg.Done()
		uri := fmt.Sprintf("/products/%s/stats", pair.ToSymbol("-", true))
		_, statErr = spot.DoRequest("GET", uri, "", &s)
	}()

	wg.Wait()

	if tickerErr != nil {
		return nil, nil, tickerErr
	}
	if statErr != nil {
		return nil, nil, statErr
	}

	now := time.Now()
	timestamp := now.UnixNano() / int64(time.Millisecond)
	datetime := now.In(spot.config.Location).Format(GO_BIRTHDAY)
	ticker := &Ticker{
		Pair:      pair,
		High:      s.High,
		Low:       s.Low,
		Last:      s.Last,
		Vol:       t.Volume,
		Buy:       t.Buy,
		Sell:      t.Sell,
		Timestamp: timestamp,
		Date:      datetime,
	}
	return ticker, tickerResp, nil
}

func (*Spot) GetDepth(pair Pair, size int) (*Depth, []byte, error) {
	panic("implement me")
}

func (spot *Spot) GetKlineRecords(pair Pair, period, size, since int) ([]*Kline, []byte, error) {
	if size > 300 {
		return nil, nil, errors.New("Can not request more than 300. ")
	}

	granularity, exist := _INERNAL_KLINE_PERIOD_CONVERTER[period]
	if !exist {
		return nil, nil, errors.New("The coinbase just support 1min 5min 15min 6h 1day. ")
	}

	uri := fmt.Sprintf(
		"/products/%s/candles?",
		pair.ToSymbol("-", true),
	)

	params := url.Values{}

	if since > 0 {
		startTimeFmt := fmt.Sprintf("%d", since)
		if len(startTimeFmt) >= 10 {
			startTimeFmt = startTimeFmt[0:10]
		}
		ts, err := strconv.ParseInt(startTimeFmt, 10, 64)
		if err != nil {
			return nil, nil, err
		}
		startTime := time.Unix(ts, 0).UTC()
		endTime := time.Unix(ts+int64(size*granularity), 0).UTC()

		params.Add("start", startTime.Format(time.RFC3339))
		params.Add("end", endTime.Format(time.RFC3339))
	}

	params.Add("granularity", fmt.Sprintf("%d", granularity))
	var response [][]interface{}
	resp, err := spot.DoRequest(
		"GET",
		uri+params.Encode(),
		"",
		&response,
	)
	if err != nil {
		return nil, nil, err
	}

	var klines []*Kline
	for _, item := range response {
		t := time.Unix(ToInt64(item[0]), 0)
		klines = append(klines, &Kline{
			Exchange:  COINBASE,
			Timestamp: t.UnixNano() / int64(time.Millisecond),
			Date:      t.In(spot.config.Location).Format(GO_BIRTHDAY),
			Pair:      pair,
			Open:      ToFloat64(item[3]),
			High:      ToFloat64(item[2]),
			Low:       ToFloat64(item[1]),
			Close:     ToFloat64(item[4]),
			Vol:       ToFloat64(item[5])},
		)
	}

	return GetAscKline(klines), resp, nil
}

func (spot *Spot) GetHistoricalCandles(pair Pair, period, size, since int) ([]*Kline, []byte, error) {
	if size > 300 {
		return nil, nil, errors.New("Can not request more than 300. ")
	}

	granularity, exist := _INERNAL_KLINE_PERIOD_CONVERTER[period]
	if !exist {
		return nil, nil, errors.New("The coinbase just support 1min 5min 15min 6h 1day. ")
	}

	uri := fmt.Sprintf(
		"/products/%s/candles?",
		pair.ToSymbol("-", true),
	)

	params := url.Values{}

	if since > 0 {
		startTimeFmt := fmt.Sprintf("%d", since)
		if len(startTimeFmt) >= 10 {
			startTimeFmt = startTimeFmt[0:10]
		}
		ts, err := strconv.ParseInt(startTimeFmt, 10, 64)
		if err != nil {
			return nil, nil, err
		}
		startTime := time.Unix(ts, 0).UTC()
		endTime := time.Unix(ts+int64(size*granularity), 0).UTC()

		params.Add("start", startTime.Format(time.RFC3339))
		params.Add("end", endTime.Format(time.RFC3339))
	}

	params.Add("granularity", fmt.Sprintf("%d", granularity))
	var response [][]interface{}
	resp, err := spot.DoRequestHistorical(
		"GET",
		uri+params.Encode(),
		"",
		&response,
	)
	if err != nil {
		return nil, nil, err
	}

	var klines []*Kline
	for _, item := range response {
		t := time.Unix(ToInt64(item[0]), 0)
		klines = append(klines, &Kline{
			Exchange:  COINBASE,
			Timestamp: t.UnixNano() / int64(time.Millisecond),
			Date:      t.In(spot.config.Location).Format(GO_BIRTHDAY),
			Pair:      pair,
			Open:      ToFloat64(item[3]),
			High:      ToFloat64(item[2]),
			Low:       ToFloat64(item[1]),
			Close:     ToFloat64(item[4]),
			Vol:       ToFloat64(item[5])},
		)
	}

	return GetAscKline(klines), resp, nil
}

func (*Spot) GetTrades(pair Pair, since int64) ([]*Trade, error) {
	panic("implement me")
}

func (*Spot) GetExchangeName() string {
	return COINBASE
}

func (spot *Spot) GetExchangeRule(pair Pair) (*Rule, []byte, error) {

	uri := fmt.Sprintf("/products/%s", pair.ToSymbol("-", true))
	r := struct {
		BaseCurrency  string  `json:"base_currency"`
		BaseIncrement float64 `json:"base_increment,string"`
		BaseMinSize   float64 `json:"base_min_size,string"`

		QuoteCurrency  string  `json:"quote_currency"`
		QuoteIncrement float64 `json:"quote_increments,string"`
	}{}

	resp, err := spot.DoRequest("GET", uri, "", &r)
	if err != nil {
		return nil, resp, err
	}

	rule := Rule{
		Pair:    pair,
		Base:    NewCurrency(r.BaseCurrency, ""),
		Counter: NewCurrency(r.QuoteCurrency, ""),

		BaseMinSize:      r.BaseMinSize,
		BasePrecision:    GetPrecision(r.BaseIncrement),
		CounterPrecision: GetPrecision(r.QuoteIncrement),
	}

	return &rule, resp, nil
}

// private api
func (*Spot) PlaceOrder(*Order) ([]byte, error) {
	panic("implement me")
}

func (*Spot) CancelOrder(*Order) ([]byte, error) {
	panic("implement me")
}

func (*Spot) GetOrder(*Order) ([]byte, error) {
	panic("implement me")
}

func (*Spot) GetOrders(pair Pair) ([]*Order, error) {
	panic("implement me")
}

func (*Spot) GetUnFinishOrders(pair Pair) ([]*Order, []byte, error) {
	panic("implement me")
}

func (*Spot) GetAccount() (*Account, []byte, error) {
	panic("implement me")
}

// util api
func (spot *Spot) KeepAlive() {
	nowTimestamp := time.Now().Unix() * 1000
	if (nowTimestamp - spot.config.LastTimestamp) < 5*1000 {
		return
	}
	_, _, _ = spot.GetExchangeRule(Pair{Basis: BTC, Counter: USD})
}

func (spot *Spot) GetOHLCs(symbol string, period, size, since int) ([]*OHLC, []byte, error) {
	panic("implement me")
}
