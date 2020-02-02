package coinbase

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	. "github.com/strengthening/goghostex"
)

type Spot struct {
	*Coinbase
}

func (*Spot) LimitBuy(*Order) ([]byte, error) {
	panic("implement me")
}

func (*Spot) LimitSell(*Order) ([]byte, error) {
	panic("implement me")
}

func (*Spot) MarketBuy(*Order) ([]byte, error) {
	panic("implement me")
}

func (*Spot) MarketSell(*Order) ([]byte, error) {
	panic("implement me")
}

func (*Spot) CancelOrder(*Order) ([]byte, error) {
	panic("implement me")
}

func (*Spot) GetOneOrder(*Order) ([]byte, error) {
	panic("implement me")
}

func (*Spot) GetUnFinishOrders(currency CurrencyPair) ([]Order, []byte, error) {
	panic("implement me")
}

func (*Spot) GetOrderHistorys(currency CurrencyPair, currentPage, pageSize int) ([]Order, error) {
	panic("implement me")
}

func (*Spot) GetAccount() (*Account, []byte, error) {
	panic("implement me")
}

func (*Spot) GetTicker(currency CurrencyPair) (*Ticker, []byte, error) {
	panic("implement me")
}

func (*Spot) GetDepth(size int, currency CurrencyPair) (*Depth, []byte, error) {
	panic("implement me")
}

func (spot *Spot) GetKlineRecords(currency CurrencyPair, period, size, since int) ([]Kline, []byte, error) {
	if size > 300 {
		return nil, nil, errors.New("Can not request more than 300. ")
	}

	granularity, exist := _INERNAL_KLINE_PERIOD_CONVERTER[period]
	if !exist {
		return nil, nil, errors.New("The coinbase just support 1min 5min 15min 6h 1day. ")
	}

	uri := fmt.Sprintf(
		"/products/%s/candles?",
		currency.ToSymbol("-"),
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

	var klines []Kline
	for _, item := range response {
		t := time.Unix(ToInt64(item[0]), 0)
		klines = append(klines, Kline{
			Exchange:  COINBASE,
			Timestamp: t.UnixNano() / int64(time.Millisecond),
			Date:      t.In(spot.config.Location).Format(GO_BIRTHDAY),
			Pair:      currency,
			Open:      ToFloat64(item[3]),
			High:      ToFloat64(item[2]),
			Low:       ToFloat64(item[1]),
			Close:     ToFloat64(item[4]),
			Vol:       ToFloat64(item[5])},
		)
	}

	return klines, resp, nil
}

func (*Spot) GetTrades(currencyPair CurrencyPair, since int64) ([]Trade, error) {
	panic("implement me")
}

func (*Spot) GetExchangeName() string {
	return COINBASE
}