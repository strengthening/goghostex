package coinbase

import (
	"encoding/json"
	"time"

	. "github.com/deforceHK/goghostex"
)

const (
	ACCEPT        = "Accept"
	CONTENT_TYPE  = "Content-Type"
	CACHE_CONTROL = "Cache-control"

	APPLICATION_JSON      = "application/json"
	APPLICATION_JSON_UTF8 = "application/json; charset=UTF-8"

	ENDPOINT = "https://api.exchange.coinbase.com"
)

type Coinbase struct {
	config *APIConfig
	Spot   *Spot
	//Future *Future
	//Margin *Margin
	//Wallet *Wallet
}

var _INERNAL_KLINE_PERIOD_CONVERTER = map[int]int{
	KLINE_PERIOD_1MIN:  60,
	KLINE_PERIOD_5MIN:  300,
	KLINE_PERIOD_15MIN: 900,
	KLINE_PERIOD_6H:    21600,
	KLINE_PERIOD_1DAY:  86400,
}

func New(config *APIConfig) *Coinbase {
	cb := &Coinbase{config: config}
	cb.Spot = &Spot{cb}

	return cb
}

func (coinbase *Coinbase) DoRequest(
	httpMethod,
	uri,
	reqBody string,
	response interface{},
) ([]byte, error) {

	url := coinbase.config.Endpoint + uri
	resp, err := NewHttpRequest(
		coinbase.config.HttpClient,
		httpMethod,
		url,
		reqBody,
		map[string]string{
			CONTENT_TYPE:  APPLICATION_JSON_UTF8,
			ACCEPT:        APPLICATION_JSON,
			CACHE_CONTROL: "no-store", // test to not use cached for coinbase
		},
	)

	if err != nil {
		return nil, err
	} else {
		nowTimestamp := time.Now().Unix() * 1000
		if nowTimestamp > coinbase.config.LastTimestamp {
			coinbase.config.LastTimestamp = nowTimestamp
		}
		return resp, json.Unmarshal(resp, &response)
	}
}

// coinbase can request historical candles data from https://api.pro.coinbase.com/

func (coinbase *Coinbase) DoRequestHistorical(
	httpMethod,
	uri,
	reqBody string,
	response interface{},
) ([]byte, error) {

	url := "https://api.pro.coinbase.com" + uri
	resp, err := NewHttpRequest(
		coinbase.config.HttpClient,
		httpMethod,
		url,
		reqBody,
		map[string]string{
			CONTENT_TYPE:  APPLICATION_JSON_UTF8,
			ACCEPT:        APPLICATION_JSON,
			CACHE_CONTROL: "no-store", // test to not use cached for coinbase
		},
	)

	if err != nil {
		return nil, err
	} else {
		nowTimestamp := time.Now().Unix() * 1000
		if nowTimestamp > coinbase.config.LastTimestamp {
			coinbase.config.LastTimestamp = nowTimestamp
		}
		return resp, json.Unmarshal(resp, &response)
	}
}
