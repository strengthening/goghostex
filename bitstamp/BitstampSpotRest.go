package bitstamp

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	. "github.com/strengthening/goghostex"
)

type Spot struct {
	*Bitstamp
}

func (this *Spot) LimitBuy(*Order) ([]byte, error) {
	panic("implement me")
}

func (this *Spot) LimitSell(*Order) ([]byte, error) {
	panic("implement me")
}

func (this *Spot) MarketBuy(*Order) ([]byte, error) {
	panic("implement me")
}

func (this *Spot) MarketSell(*Order) ([]byte, error) {
	panic("implement me")
}

func (this *Spot) CancelOrder(*Order) ([]byte, error) {
	panic("implement me")
}

func (this *Spot) GetOneOrder(*Order) ([]byte, error) {
	panic("implement me")
}

func (this *Spot) GetUnFinishOrders(pair Pair) ([]Order, []byte, error) {
	panic("implement me")
}

func (this *Spot) GetOrderHistorys(pair Pair, currentPage, pageSize int) ([]Order, error) {
	panic("implement me")
}

func (this *Spot) GetTrades(pair Pair, since int64) ([]Trade, error) {
	panic("implement me")
}

func (this *Spot) GetExchangeName() string {
	panic("implement me")
}

func (this *Spot) GetUnfinishOrders(pair Pair) ([]Order, []byte, error) {
	panic("implement me")
}

func (this *Spot) GetAccount() (*Account, []byte, error) {
	panic("implement me")
}

func (this *Spot) GetTicker(pair Pair) (*Ticker, []byte, error) {

	uri := "/api/v2/ticker/" + pair.ToSymbol("", false)
	response := struct {
		High      float64 `json:"high,string"`
		Low       float64 `json:"low,string"`
		Last      float64 `json:"last,string"`
		Buy       float64 `json:"bid,string"`
		Sell      float64 `json:"ask,string"`
		Volume    float64 `json:"volume,string"`
		Timestamp float64 `json:"timestamp,string"`
	}{}

	resp, err := this.DoRequest("GET", uri, "", &response)
	if err != nil {
		return nil, nil, err
	}

	return &Ticker{
		Pair:      pair,
		Last:      ToFloat64(response.Last),
		High:      ToFloat64(response.High),
		Low:       ToFloat64(response.Low),
		Vol:       ToFloat64(response.Volume),
		Sell:      ToFloat64(response.Sell),
		Buy:       ToFloat64(response.Buy),
		Timestamp: int64(response.Timestamp) * 1000,
		Date: time.Unix(
			int64(response.Timestamp),
			0,
		).In(this.config.Location).Format(GO_BIRTHDAY),
	}, resp, nil
}

func (this *Spot) GetDepth(size int, pair Pair) (*Depth, []byte, error) {
	uri := "/api/v2/order_book/" + pair.ToSymbol("", false)
	response := struct {
		Bids      [][]interface{} `json:"bids"`
		Asks      [][]interface{} `json:"asks"`
		Status    string          `json:"status"`
		Reason    string          `json:"reason"`
		Timestamp int64           `json:"timestamp,string"`
	}{}

	resp, err := this.DoRequest("GET", uri, "", &response) //&response)
	if err != nil {
		return nil, nil, err
	}

	dep := new(Depth)
	dep.Pair = pair
	dep.Timestamp = response.Timestamp * 1000
	dep.Date = time.Unix(
		int64(response.Timestamp)/1000,
		0,
	).In(this.config.Location).Format(GO_BIRTHDAY)

	for _, itm := range response.Asks {
		dep.AskList = append(dep.AskList, DepthRecord{
			Price:  ToFloat64(itm[0]),
			Amount: ToFloat64(itm[1]),
		})
	}

	for _, itm := range response.Bids {
		dep.BidList = append(dep.BidList, DepthRecord{
			Price:  ToFloat64(itm[0]),
			Amount: ToFloat64(itm[1]),
		})
	}

	sort.Sort(sort.Reverse(dep.AskList)) //reverse
	return dep, resp, nil
}

// bitstamp kline api can only return the nearly hour data. Cause it's api design.
func (this *Spot) GetKlineRecords(pair Pair, period, size, since int) ([]Kline, []byte, error) {
	if period != KLINE_PERIOD_1MIN {
		return nil, nil, errors.New("Can not support the period in bitstamp. ")
	}

	uri := fmt.Sprintf(
		"/api/v2/transactions/%s/?time=day",
		strings.ToLower(pair.ToSymbol("", false)),
	)
	response := make([]struct {
		Date   int64   `json:"date,string"`
		Price  float64 `json:"price,string"`
		Amount float64 `json:"amount,string"`
	}, 0)

	resp, err := this.DoRequest("GET", uri, "", &response) //&response)
	if err != nil {
		return nil, nil, err
	}
	if len(response) == 0 {
		return nil, nil, errors.New("Have not receive enough data. ")
	}

	klineRecord := make(map[int64]Kline, 0)
	klineTimestamp := make([]int64, 0)
	for _, order := range response {
		minTimestamp := order.Date / 60 * 60 * 1000
		kline, exist := klineRecord[minTimestamp]
		if !exist {
			t := time.Unix(minTimestamp/1000, 0)
			kline = Kline{
				Timestamp: minTimestamp,
				Date:      t.In(this.config.Location).Format(GO_BIRTHDAY),
				Pair:      pair,
				Exchange:  BITSTAMP,
				Open:      order.Price,
				High:      order.Price,
				Low:       order.Price,
				Close:     order.Price,
				Vol:       order.Amount,
			}
			klineRecord[minTimestamp] = kline
			klineTimestamp = append(klineTimestamp, minTimestamp)
			continue
		}

		kline.Open = order.Price
		kline.Vol += order.Amount
		if order.Price > kline.High {
			kline.High = order.Price
		}
		if order.Price < kline.Low {
			kline.Low = order.Price
		}
		klineRecord[minTimestamp] = kline
	}

	klines := make([]Kline, 0)
	for i := 0; i < len(klineTimestamp)-1; i++ {
		klines = append(klines, klineRecord[klineTimestamp[i]])
	}

	return klines, resp, nil
}
