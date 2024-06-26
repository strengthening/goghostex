package binance

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	. "github.com/deforceHK/goghostex"
)

const (
	FUTURE_CM_ENDPOINT    = "https://dapi.binance.com"
	FUTURE_UM_ENDPOINT    = "https://fapi.binance.com"
	FUTURE_KEEP_ALIVE_URI = "/dapi/v1/ping"

	FUTURE_TICKER_URI           = "/dapi/v1/ticker/24hr?"
	FUTURE_EXCHANGE_INFO_URI    = "/dapi/v1/exchangeInfo"
	FUTURE_UM_EXCHANGE_INFO_URI = "/fapi/v1/exchangeInfo"

	FUTURE_DEPTH_URI     = "/dapi/v1/depth?"
	FUTURE_KLINE_URI     = "/dapi/v1/continuousKlines?"
	FUTURE_CM_CANDLE_URI = "/dapi/v1/continuousKlines?"
	FUTURE_UM_CANDLE_URI = "/fapi/v1/continuousKlines?"
	FUTURE_TRADE_URI     = "/dapi/v1/trades?"

	FUTURE_INCOME_URI       = "/dapi/v1/income?"
	FUTURE_ACCOUNT_URI      = "/dapi/v1/account?"
	FUTURE_POSITION_URI     = "/dapi/v1/positionRisk?"
	FUTURE_PLACE_ORDER_URI  = "/dapi/v1/order?"
	FUTURE_CANCEL_ORDER_URI = "/dapi/v1/order?"
	FUTURE_GET_ORDER_URI    = "/dapi/v1/order?"
	FUTURE_GET_ORDERS_URI   = "/dapi/v1/allOrders?"
)

type Future struct {
	*Binance
	Locker                 sync.Locker
	Contracts              FutureContracts
	LastTimestamp          int64
	nextUpdateContractTime time.Time

	FutureContracts []*FutureContract
	updateTimestamp int64 // update future contracts timestamp
}

func (future *Future) GetTicker(pair Pair, contractType string) (*FutureTicker, []byte, error) {
	if contractType == THIS_WEEK_CONTRACT || contractType == NEXT_WEEK_CONTRACT {
		return nil, nil, errors.New("binance have not this_week next_week contract. ")
	}

	var contract, errContract = future.GetContract(pair, contractType)
	if errContract != nil {
		return nil, nil, errContract
	}
	var params = url.Values{}
	params.Add("symbol", future.getBNSymbol(contract.ContractName))

	var response = make([]struct {
		Symbol     string  `json:"symbol"`
		Pair       string  `json:"pair"`
		LastPrice  float64 `json:"lastPrice,string"`
		OpenPrice  float64 `json:"openPrice,string"`
		HighPrice  float64 `json:"highPrice,string"`
		LowPrice   float64 `json:"lowPrice,string"`
		Volume     float64 `json:"volume,string"`
		BaseVolume float64 `json:"baseVolume,string"`
	}, 0)

	var resp, err = future.DoRequest(
		http.MethodGet,
		FUTURE_CM_ENDPOINT,
		FUTURE_TICKER_URI+params.Encode(),
		"",
		&response,
	)
	if err != nil {
		return nil, nil, err
	}
	if len(response) == 0 {
		return nil, nil, errors.New("Can not find the pair. ")
	}

	var nowTime = time.Now()
	return &FutureTicker{
		Ticker: Ticker{
			Pair:      pair,
			Last:      response[0].LastPrice,
			Buy:       response[0].LastPrice,
			Sell:      response[0].LastPrice,
			High:      response[0].HighPrice,
			Low:       response[0].LowPrice,
			Vol:       response[0].BaseVolume,
			Timestamp: nowTime.UnixNano() / int64(time.Millisecond),
			Date:      nowTime.Format(GO_BIRTHDAY),
		},
		ContractType: contract.ContractType,
		ContractName: contract.ContractName,
	}, resp, nil
}

type bnCMContract struct {
	Symbol      string `json:"symbol"`
	Pair        string `json:"pair"`
	BaseAsset   string `json:"baseAsset"`
	QuoteAsset  string `json:"quoteAsset"`
	MarginAsset string `json:"marginAsset"`

	ContractType      string  `json:"contractType"`
	DeliveryDate      int64   `json:"deliveryDate"`
	OnboardDate       int64   `json:"onboardDate"`
	ContractStatus    string  `json:"contractStatus"`
	ContractSize      float64 `json:"contractSize"`
	PricePrecision    int64   `json:"pricePrecision"`
	QuantityPrecision int64   `json:"quantityPrecision"`

	Filters []map[string]interface{} `json:"filters"`
}

type bnUMContract struct {
	Symbol       string `json:"symbol"`
	Pair         string `json:"pair"`
	ContractType string `json:"contractType"`
	DeliveryDate int64  `json:"deliveryDate"`
	OnboardDate  int64  `json:"onboardDate"`
	Status       string `json:"status"`
	BaseAsset    string `json:"baseAsset"`
	QuoteAsset   string `json:"quoteAsset"`
	MarginAsset  string `json:"marginAsset"`

	PricePrecision    int64 `json:"pricePrecision"`
	QuantityPrecision int64 `json:"quantityPrecision"`

	Filters []map[string]interface{} `json:"filters"`
}

func (future *Future) updateContracts() {
	var nowTimestamp = time.Now().UnixNano() / int64(time.Millisecond)
	var updateTimestamp = nowTimestamp
	if future.FutureContracts != nil && nowTimestamp < future.updateTimestamp {
		return
	}

	var contracts, err = future.GetContracts()
	if err != nil {
		future.updateTimestamp = time.Now().UnixNano() / int64(time.Millisecond)
		return
	}

	var notTrading = false
	for _, c := range contracts {
		if c.Status != "TRADING" {
			notTrading = true
		}
	}
	// 如果有一个合约的状态不是TRADING，那么十分钟后再更新，此外每个整点更新一次
	if notTrading {
		updateTimestamp = nowTimestamp + 10*60*1000
	} else {
		updateTimestamp = (nowTimestamp/(60*60*1000) + 1) * 60 * 60 * 1000
	}

	future.FutureContracts = contracts
	future.updateTimestamp = updateTimestamp
}

func (future *Future) GetContracts() ([]*FutureContract, error) {
	var contracts = make([]*FutureContract, 0)

	var respCm = struct {
		Symbols    []*bnCMContract `json:"symbols"`
		ServerTime int64           `json:"serverTime"`
	}{}

	var _, errCm = future.DoRequest(
		http.MethodGet,
		FUTURE_CM_ENDPOINT,
		FUTURE_EXCHANGE_INFO_URI,
		"",
		&respCm,
	)
	if errCm != nil {
		return nil, errCm
	}

	var nowTimestamp = time.Now().UnixNano() / int64(time.Millisecond)
	for _, item := range respCm.Symbols {
		// it is not future , it's swap in this project.
		if strings.Contains(item.ContractType, "PERPETUAL") ||
			item.DeliveryDate > (nowTimestamp+5*365*24*60*60*1000) {
			continue
		}

		var priceMaxScale, priceMinScale = float64(1.2), float64(0.8)
		var tickSize = float64(-1)
		for _, filter := range item.Filters {
			if value, ok := filter["filterType"].(string); ok && value == "PERCENT_PRICE" {
				priceMaxScale = ToFloat64(filter["multiplierUp"])
				priceMinScale = ToFloat64(filter["multiplierDown"])
			}

			if value, ok := filter["filterType"].(string); ok && value == "PRICE_FILTER" {
				tickSize = ToFloat64(filter["tickSize"])
			}
		}

		var dueTime = time.Unix(item.DeliveryDate/1000, 0).In(future.config.Location)
		var openTime = time.Unix(item.OnboardDate/1000, 0).In(future.config.Location)
		var listTime = time.Unix(item.OnboardDate/1000, 0).In(future.config.Location)

		var pair = Pair{
			Basis:   NewCurrency(item.BaseAsset, ""),
			Counter: NewCurrency(item.QuoteAsset, ""),
		}

		//var contractNameInfo = strings.Split(item.Symbol, "_")
		var contract = &FutureContract{
			Pair:         pair,
			Symbol:       pair.ToSymbol("_", false),
			Exchange:     BINANCE,
			ContractType: item.ContractType,
			ContractName: item.Symbol,
			Type:         FUTURE_TYPE_INVERSER, // "inverse", "linear

			SettleMode:    SETTLE_MODE_BASIS,
			Status:        item.ContractStatus,
			OpenTimestamp: openTime.UnixNano() / int64(time.Millisecond),
			OpenDate:      openTime.Format(GO_BIRTHDAY),
			ListTimestamp: listTime.UnixNano() / int64(time.Millisecond),
			ListDate:      listTime.Format(GO_BIRTHDAY),
			DueTimestamp:  dueTime.UnixNano() / int64(time.Millisecond),
			DueDate:       dueTime.Format(GO_BIRTHDAY),

			UnitAmount:      item.ContractSize,
			TickSize:        tickSize,
			PricePrecision:  item.PricePrecision,
			AmountPrecision: item.QuantityPrecision,

			MaxScalePriceLimit: priceMaxScale,
			MinScalePriceLimit: priceMinScale,
		}

		contracts = append(contracts, contract)
	}

	var respUm = struct {
		Symbols    []*bnUMContract `json:"symbols"`
		ServerTime int64           `json:"serverTime"`
	}{}

	var _, errUm = future.DoRequest(
		http.MethodGet,
		FUTURE_UM_ENDPOINT,
		FUTURE_UM_EXCHANGE_INFO_URI,
		"",
		&respUm,
	)
	if errUm != nil {
		return nil, errUm
	}

	for _, item := range respUm.Symbols {
		if strings.Contains(item.ContractType, "PERPETUAL") ||
			item.ContractType == "" ||
			item.DeliveryDate > (nowTimestamp+5*365*24*60*60*1000) {
			continue
		}

		var priceMaxScale, priceMinScale float64 = 1.2, 0.8
		var tickSize float64 = -1
		for _, filter := range item.Filters {
			if value, ok := filter["filterType"].(string); ok && value == "PERCENT_PRICE" {
				priceMaxScale = ToFloat64(filter["multiplierUp"])
				priceMinScale = ToFloat64(filter["multiplierDown"])
			}

			if value, ok := filter["filterType"].(string); ok && value == "PRICE_FILTER" {
				tickSize = ToFloat64(filter["tickSize"])
			}
		}

		var dueTime = time.Unix(item.DeliveryDate/1000, 0).In(future.config.Location)
		var openTime = time.Unix(item.OnboardDate/1000, 0).In(future.config.Location)
		var listTime = time.Unix(item.OnboardDate/1000, 0).In(future.config.Location)

		var pair = Pair{
			Basis:   NewCurrency(item.BaseAsset, ""),
			Counter: NewCurrency(item.QuoteAsset, ""),
		}

		var contract = &FutureContract{
			Pair:         pair,
			Symbol:       pair.ToSymbol("_", false),
			Exchange:     BINANCE,
			ContractType: item.ContractType,
			ContractName: item.Symbol,
			Type:         FUTURE_TYPE_LINEAR, // "inverse", "linear

			SettleMode: SETTLE_MODE_COUNTER,
			Status:     item.Status,

			OpenTimestamp: openTime.UnixNano() / int64(time.Millisecond),
			OpenDate:      openTime.Format(GO_BIRTHDAY),

			ListTimestamp: listTime.UnixNano() / int64(time.Millisecond),
			ListDate:      listTime.Format(GO_BIRTHDAY),
			DueTimestamp:  dueTime.UnixNano() / int64(time.Millisecond),
			DueDate:       dueTime.Format(GO_BIRTHDAY),

			UnitAmount:      1,
			TickSize:        tickSize,
			PricePrecision:  item.PricePrecision,
			AmountPrecision: item.QuantityPrecision,

			MaxScalePriceLimit: priceMaxScale,
			MinScalePriceLimit: priceMinScale,
		}

		contracts = append(contracts, contract)
	}

	return contracts, nil
}

func (future *Future) GetContract(pair Pair, contractType string) (*FutureContract, error) {
	return future.getFutureContract(pair, contractType)
}

func (future *Future) GetDepth(pair Pair, contractType string, size int) (*FutureDepth, []byte, error) {
	if contractType == THIS_WEEK_CONTRACT || contractType == NEXT_WEEK_CONTRACT {
		return nil, nil, errors.New("binance have not this_week next_week contract. ")
	}
	var contract, err = future.GetContract(pair, contractType)
	if err != nil {
		return nil, nil, err
	}

	var params = url.Values{}
	params.Add("symbol", future.getBNSymbol(contract.ContractName))
	params.Add("limit", fmt.Sprintf("%d", size))

	response := struct {
		LastUpdateId int64      `json:"lastUpdateId"`
		E            int64      `json:"E"`
		T            int64      `json:"T"`
		Bids         [][]string `json:"bids"`
		Asks         [][]string `json:"asks"`
	}{}

	resp, err := future.DoRequest(
		http.MethodGet,
		FUTURE_CM_ENDPOINT,
		FUTURE_DEPTH_URI+params.Encode(),
		"",
		&response,
	)
	if err != nil {
		return nil, resp, err
	} else {
		dateFmt := time.Unix(response.E/1000, response.E%1000).In(future.config.Location).Format(GO_BIRTHDAY)
		depth := FutureDepth{
			ContractType: contract.ContractType,
			ContractName: contract.ContractName,
			Pair:         pair,
			Timestamp:    response.E,
			DueTimestamp: contract.DueTimestamp,
			Sequence:     response.LastUpdateId,
			Date:         dateFmt,
			AskList:      DepthRecords{},
			BidList:      DepthRecords{},
		}

		for _, items := range response.Asks {
			depth.AskList = append(depth.AskList, DepthRecord{Price: ToFloat64(items[0]), Amount: ToFloat64(items[1])})
		}
		for _, items := range response.Bids {
			depth.BidList = append(depth.BidList, DepthRecord{Price: ToFloat64(items[0]), Amount: ToFloat64(items[1])})
		}
		return &depth, resp, nil
	}
}

func (future *Future) GetLimit(pair Pair, contractType string) (float64, float64, error) {
	if contractType == THIS_WEEK_CONTRACT || contractType == NEXT_WEEK_CONTRACT {
		return 0, 0, errors.New("binance have not this_week next_week contract. ")
	}

	var contract, err = future.GetContract(pair, contractType)
	if err != nil {
		return 0, 0, err
	}

	var bnSymbol = future.getBNSymbol(contract.ContractName)
	var response = make([]struct {
		Symbol string  `json:"symbol"`
		Price  float64 `json:"markPrice,string"` //  mark price
	}, 0)

	if _, err := future.DoRequest(
		http.MethodGet,
		FUTURE_CM_ENDPOINT,
		fmt.Sprintf("/dapi/v1/premiumIndex?symbol=%s", bnSymbol),
		"",
		&response,
	); err != nil {
		return 0, 0, err
	}
	if len(response) == 0 {
		return 0, 0, errors.New("the remote return no data. ")
	}

	var highLimit = response[0].Price * contract.MaxScalePriceLimit
	var lowLimit = response[0].Price * contract.MinScalePriceLimit
	return highLimit, lowLimit, nil
}

func (future *Future) GetIndex(pair Pair) (float64, []byte, error) {
	panic("implement me")
}

func (future *Future) GetMark(pair Pair, contractType string) (float64, []byte, error) {
	if contractType == THIS_WEEK_CONTRACT || contractType == NEXT_WEEK_CONTRACT {
		return 0, nil, errors.New("binance have not this_week next_week contract. ")
	}
	var contract, errContract = future.GetContract(pair, contractType)
	if errContract != nil {
		return 0, nil, errContract
	}

	var bnSymbol = future.getBNSymbol(contract.ContractName)
	var response = make([]struct {
		Symbol string  `json:"symbol"`
		Price  float64 `json:"markPrice,string"` //  mark price
	}, 0)
	var resp, err = future.DoRequest(
		http.MethodGet,
		FUTURE_CM_ENDPOINT,
		fmt.Sprintf("/dapi/v1/premiumIndex?symbol=%s", bnSymbol),
		"",
		&response,
	)
	if err != nil {
		return 0, resp, err
	}
	if len(response) == 0 {
		return 0, resp, errors.New("the remote return no data. ")
	}
	return response[0].Price, resp, nil
}

func (future *Future) GetKlineRecords(
	contractType string,
	pair Pair,
	period, size, since int,
) ([]*FutureKline, []byte, error) {
	if contractType == THIS_WEEK_CONTRACT || contractType == NEXT_WEEK_CONTRACT {
		return nil, nil, errors.New("binance have not the this_week next_week contract. ")
	}

	var endTimestamp = since + size*_INERNAL_KLINE_SECOND_CONVERTER[period]
	if endTimestamp > since+200*24*60*60*1000 {
		endTimestamp = since + 200*24*60*60*1000
	}
	if endTimestamp > int(time.Now().Unix()*1000) {
		endTimestamp = int(time.Now().Unix() * 1000)
	}
	var paramContractType = "CURRENT_QUARTER"
	if contractType == NEXT_QUARTER_CONTRACT {
		paramContractType = "NEXT_QUARTER"
	}

	params := url.Values{}
	params.Set("pair", pair.ToSymbol("", true))
	params.Set("contractType", paramContractType)
	params.Set("interval", _INERNAL_KLINE_PERIOD_CONVERTER[period])
	params.Set("startTime", fmt.Sprintf("%d", since))
	params.Set("endTime", fmt.Sprintf("%d", endTimestamp))
	params.Set("limit", fmt.Sprintf("%d", size))

	uri := FUTURE_KLINE_URI + params.Encode()
	klines := make([][]interface{}, 0)
	resp, err := future.DoRequest(
		http.MethodGet,
		FUTURE_CM_ENDPOINT,
		uri,
		"",
		&klines,
	)
	if err != nil {
		return nil, resp, err
	}

	var list []*FutureKline
	for _, k := range klines {
		var timestamp = ToInt64(k[0])
		var _, dueBoard = GetDueTimestamp(timestamp)
		var dueTimestamp = dueBoard[contractType]
		var dueDate = time.Unix(dueTimestamp/1000, 0).In(future.config.Location).Format(GO_BIRTHDAY)
		var r = &FutureKline{
			Kline: Kline{
				Pair:      pair,
				Exchange:  BINANCE,
				Timestamp: timestamp,
				Date:      time.Unix(timestamp/1000, 0).In(future.config.Location).Format(GO_BIRTHDAY),
				Open:      ToFloat64(k[1]),
				High:      ToFloat64(k[2]),
				Low:       ToFloat64(k[3]),
				Close:     ToFloat64(k[4]),
				Vol:       ToFloat64(k[7]),
			},
			ContractType: contractType,
			DueTimestamp: dueTimestamp,
			DueDate:      dueDate,
			Vol2:         ToFloat64(k[5]),
		}
		list = append(list, r)
	}
	return GetAscFutureKline(list), resp, nil
}

func (future *Future) GetCandles(
	dueTimestamp int64,
	symbol string,
	period,
	size int,
	since int64,
) ([]*FutureCandle, []byte, error) {
	future.updateContracts()
	if future.FutureContracts == nil {
		return nil, nil, errors.New("future contracts have not update. ")
	}

	var contract *FutureContract = nil
	for _, c := range future.FutureContracts {
		if c.Symbol == symbol && c.DueTimestamp == dueTimestamp {
			contract = c
			break
		}
	}
	if contract == nil {
		return nil, nil, errors.New("the contract not found. ")
	}

	if contract.Type == FUTURE_TYPE_LINEAR {
		return future.getUMCandles(contract, period, size, since)
	} else {
		return future.getCMCandles(contract, period, size, since)
	}
}

func (future *Future) GetTrades(pair Pair, contractType string) ([]*Trade, []byte, error) {
	if contractType == THIS_WEEK_CONTRACT || contractType == NEXT_WEEK_CONTRACT {
		return nil, nil, errors.New("binance have not this_week next_week contract. ")
	}

	contract, err := future.GetContract(pair, contractType)
	if err != nil {
		return nil, nil, err
	}

	params := url.Values{}
	params.Set("symbol", future.getBNSymbol(contract.ContractName))

	uri := FUTURE_TRADE_URI + params.Encode()
	response := make([]struct {
		Id           int64   `json:"id"`
		Price        float64 `json:"price,string"`
		Qty          int64   `json:"qty,string"`
		BaseQty      float64 `json:"baseQty,string"`
		Time         int64   `json:"time"`
		IsBuyerMaker bool    `json:"isBuyerMaker"`
	}, 0)
	resp, err := future.DoRequest(
		http.MethodGet,
		FUTURE_CM_ENDPOINT,
		uri,
		"",
		&response,
	)
	if err != nil {
		return nil, resp, err
	}

	trades := make([]*Trade, 0)
	for _, item := range response {
		tradeType := BUY
		if !item.IsBuyerMaker {
			tradeType = SELL
		}
		trade := Trade{
			Tid:       item.Id,
			Type:      tradeType,
			Amount:    item.BaseQty,
			Price:     item.Price,
			Timestamp: item.Time,
			Pair:      pair,
		}
		trades = append(trades, &trade)
	}

	return trades, resp, nil
}

func (future *Future) GetAccount() (*FutureAccount, []byte, error) {
	params := url.Values{}
	if err := future.buildParamsSigned(&params); err != nil {
		return nil, nil, err
	}

	response := struct {
		Asset []struct {
			Asset                  string  `json:"asset"`
			WalletBalance          float64 `json:"walletBalance,string"`
			UnrealizedProfit       float64 `json:"unrealizedProfit,string"`
			MarginBalance          float64 `json:"marginBalance,string"`
			MaintMargin            float64 `json:"maintMargin,string"`
			InitialMargin          float64 `json:"initialMargin,string"`
			PositionInitialMargin  float64 `json:"positionInitialMargin,string"`
			OpenOrderInitialMargin float64 `json:"openOrderInitialMargin,string"`
			MaxWithdrawAmount      float64 `json:"maxWithdrawAmount,string"`
			CrossWalletBalance     float64 `json:"crossWalletBalance,string"`
			CrossUnPnl             float64 `json:"crossUnPnl,string"`
			AvailableBalance       float64 `json:"availableBalance,string"`
		} `json:"assets"`
	}{}

	resp, err := future.DoRequest(
		http.MethodGet,
		FUTURE_CM_ENDPOINT,
		"/dapi/v1/account?"+params.Encode(),
		"", &response,
	)
	if err != nil {
		return nil, resp, err
	}

	futureAccount := FutureAccount{
		SubAccount: make(map[Currency]FutureSubAccount, 0),
		Exchange:   BINANCE,
	}

	for _, item := range response.Asset {
		currency := NewCurrency(item.Asset, "")
		marginRate := float64(0.0)
		if item.MarginBalance > 0 {
			marginRate = item.MaintMargin / item.MarginBalance
		}

		futureAccount.SubAccount[currency] = FutureSubAccount{
			Currency: currency,

			Margin:         item.MarginBalance,
			MarginDealed:   item.PositionInitialMargin,
			MarginUnDealed: item.OpenOrderInitialMargin,
			MarginRate:     marginRate,

			BalanceTotal: item.WalletBalance,
			BalanceNet:   item.WalletBalance + item.UnrealizedProfit,
			BalanceAvail: item.MaxWithdrawAmount,

			ProfitReal:   0,
			ProfitUnreal: item.UnrealizedProfit,
		}
	}

	return &futureAccount, resp, nil
}

func (future *Future) PlaceOrder(order *FutureOrder) ([]byte, error) {
	if order == nil {
		return nil, errors.New("ord param is nil")
	}

	contract, err := future.GetContract(order.Pair, order.ContractType)
	if err != nil {
		return nil, err
	}

	side, positionSide, placeType := "", "", ""
	exist := false
	if side, exist = sideRelation[order.Type]; !exist {
		return nil, errors.New("future type not found. ")
	}
	if positionSide, exist = positionSideRelation[order.Type]; !exist {
		return nil, errors.New("future type not found. ")
	}
	if placeType, exist = placeTypeRelation[order.PlaceType]; !exist {
		return nil, errors.New("place type not found. ")
	}

	param := url.Values{}
	param.Set("symbol", future.getBNSymbol(contract.ContractName))
	param.Set("side", side)
	param.Set("positionSide", positionSide)
	param.Set("type", "LIMIT")
	param.Set("price", FloatToPrice(order.Price, contract.PricePrecision, contract.TickSize))
	param.Set("quantity", fmt.Sprintf("%d", order.Amount))
	// "GTC": 成交为止, 一直有效。
	// "IOC": 无法立即成交(吃单)的部分就撤销。
	// "FOK": 无法全部立即成交就撤销。
	// "GTX": 无法成为挂单方就撤销。
	param.Set("timeInForce", placeType)
	if order.Cid != "" {
		param.Set("newClientOrderId", order.Cid)
	}
	if err := future.buildParamsSigned(&param); err != nil {
		return nil, err
	}

	var response struct {
		Cid        string  `json:"clientOrderId"`
		Status     string  `json:"status"`
		OrderId    int64   `json:"orderId"`
		UpdateTime int64   `json:"updateTime"`
		Price      float64 `json:"price,string"`
		AvgPrice   float64 `json:"avgPrice,string"`
		Amount     int64   `json:"origQty,string"`
		DealAmount int64   `json:"executedQty,string"`
	}

	now := time.Now()
	resp, err := future.DoRequest(
		http.MethodPost,
		FUTURE_CM_ENDPOINT,
		FUTURE_PLACE_ORDER_URI+param.Encode(),
		"",
		&response,
	)
	if err != nil {
		return nil, err
	}
	orderTime := time.Unix(response.UpdateTime/1000, 0)

	order.OrderId = fmt.Sprintf("%d", response.OrderId)
	order.PlaceTimestamp = now.UnixNano() / int64(time.Millisecond)
	order.PlaceDatetime = now.In(future.config.Location).Format(GO_BIRTHDAY)
	order.DealTimestamp = response.UpdateTime
	order.DealDatetime = orderTime.In(future.config.Location).Format(GO_BIRTHDAY)
	order.Status = statusRelation[response.Status]
	order.Price = response.Price
	order.Amount = response.Amount
	order.ContractName = contract.ContractName

	if response.Cid != "" {
		order.Cid = response.Cid
	}
	if response.DealAmount > 0 {
		order.AvgPrice = response.AvgPrice
		order.DealAmount = response.DealAmount
	}

	return resp, nil
}

func (future *Future) CancelOrder(order *FutureOrder) ([]byte, error) {
	contract, err := future.GetContract(order.Pair, order.ContractType)
	if err != nil {
		return nil, err
	}

	if order.OrderId == "" && order.Cid == "" {
		return nil, errors.New("The order_id and cid is empty. ")
	}

	param := url.Values{}
	param.Set("symbol", future.getBNSymbol(contract.ContractName))
	if order.OrderId != "" {
		param.Set("orderId", order.OrderId)
	} else {
		param.Set("origClientOrderId", order.Cid)
	}
	if err := future.buildParamsSigned(&param); err != nil {
		return nil, err
	}

	var response struct {
		Cid        string  `json:"clientOrderId"`
		Status     string  `json:"status"`
		DealAmount int64   `json:"executedQty,string"`
		AvgPrice   float64 `json:"avgPrice,string"`
		OrderId    int64   `json:"orderId"`
		UpdateTime int64   `json:"updateTime"`
	}

	now := time.Now()
	resp, err := future.DoRequest(
		http.MethodDelete,
		FUTURE_CM_ENDPOINT,
		FUTURE_CANCEL_ORDER_URI+param.Encode(),
		"",
		&response,
	)
	if err != nil {
		return nil, err
	}

	orderTime := time.Unix(response.UpdateTime/1000, 0)
	order.PlaceTimestamp = now.UnixNano() / int64(time.Millisecond)
	order.PlaceDatetime = now.In(future.config.Location).Format(GO_BIRTHDAY)
	order.DealTimestamp = response.UpdateTime
	order.DealDatetime = orderTime.In(future.config.Location).Format(GO_BIRTHDAY)
	order.Status = statusRelation[response.Status]
	if response.DealAmount > 0 {
		order.AvgPrice = response.AvgPrice
		order.DealAmount = response.DealAmount
	}
	return resp, nil
}

func (future *Future) GetOrders(pair Pair, contractType string) ([]*FutureOrder, []byte, error) {
	contract, err := future.GetContract(pair, contractType)
	if err != nil {
		return nil, nil, err
	}

	var param = url.Values{}
	param.Set("symbol", future.getBNSymbol(contract.ContractName))

	if err := future.buildParamsSigned(&param); err != nil {
		return nil, nil, err
	}

	response := make([]struct {
		AvgPrice      float64 `json:"avgPrice,string"`
		ClientOrderId string  `json:"clientOrderId"`
		ExecutedQty   int64   `json:"executedQty"`
		OrderId       int64   `json:"orderId"`
		OrigQty       float64 `json:"origQty,string"`
		Price         float64 `json:"price,string"`
		Side          string  `json:"side"`
		PositionSide  string  `json:"positionSide"`
		Status        string  `json:"status"`
		Symbol        string  `json:"symbol"`
		Pair          string  `json:"pair"`
		Time          int64   `json:"time"`
		TimeInForce   string  `json:"timeInForce"`
		UpdateTime    int64   `json:"updateTime"`
	}, 0)

	resp, err := future.DoRequest(
		http.MethodGet,
		FUTURE_CM_ENDPOINT,
		FUTURE_GET_ORDERS_URI+param.Encode(),
		"",
		&response,
	)
	if err != nil {
		return nil, resp, err
	}

	orders := make([]*FutureOrder, 0)
	for _, item := range response {
		placeTime := time.Unix(item.Time/1000, item.Time%1000).In(future.config.Location)
		updateTime := time.Unix(item.UpdateTime/1000, item.UpdateTime%1000).In(future.config.Location)

		order := FutureOrder{
			Cid:            item.ClientOrderId,
			OrderId:        fmt.Sprintf("%d", item.OrderId),
			Price:          item.Price,
			AvgPrice:       item.AvgPrice,
			Amount:         ToInt64(item.OrigQty),
			DealAmount:     item.ExecutedQty,
			PlaceTimestamp: placeTime.UnixNano() / int64(time.Millisecond),
			PlaceDatetime:  placeTime.Format(GO_BIRTHDAY),
			DealTimestamp:  updateTime.UnixNano() / int64(time.Millisecond),
			DealDatetime:   updateTime.Format(GO_BIRTHDAY),
			Status:         _INTERNAL_ORDER_STATUS_REVERSE_CONVERTER[item.Status],
			PlaceType:      _INTERNAL_PLACE_TYPE_REVERSE_CONVERTER[item.TimeInForce],
			Type:           future.getFutureType(item.Side, item.PositionSide),
			//LeverRate: item.,
			//Fee:item.,
			Pair:         pair,
			ContractType: contractType,
			ContractName: contract.ContractName,
			Exchange:     BINANCE,
		}
		orders = append(orders, &order)
	}
	return orders, resp, nil
}

func (future *Future) GetOrder(order *FutureOrder) ([]byte, error) {
	if order.OrderId == "" && order.Cid == "" {
		return nil, errors.New("The order id and cid is empty. ")
	}

	contract, err := future.GetContract(order.Pair, order.ContractType)
	if err != nil {
		return nil, err
	}

	var params = url.Values{}
	params.Add("symbol", future.getBNSymbol(contract.ContractName))

	if order.OrderId != "" {
		params.Set("orderId", order.OrderId)
	} else {
		params.Set("origClientOrderId", order.Cid)
	}
	if err := future.buildParamsSigned(&params); err != nil {
		return nil, err
	}

	var response struct {
		Cid    string `json:"clientOrderId"`
		Status string `json:"status"`

		Price    float64 `json:"price,string"`
		AvgPrice float64 `json:"avgPrice,string"`

		Amount     int64 `json:"origQty,string"`
		DealAmount int64 `json:"executedQty,string"`

		OrderId    int64 `json:"orderId"`
		UpdateTime int64 `json:"updateTime"`
	}

	now := time.Now()
	resp, err := future.DoRequest(
		http.MethodGet,
		FUTURE_CM_ENDPOINT,
		FUTURE_GET_ORDER_URI+params.Encode(),
		"",
		&response,
	)
	if err != nil {
		return nil, err
	}

	orderTime := time.Unix(response.UpdateTime/1000, 0)
	order.PlaceTimestamp = now.UnixNano() / int64(time.Millisecond)
	order.PlaceDatetime = now.In(future.config.Location).Format(GO_BIRTHDAY)
	order.DealTimestamp = response.UpdateTime
	order.DealDatetime = orderTime.In(future.config.Location).Format(GO_BIRTHDAY)
	order.Status = statusRelation[response.Status]
	order.Price = response.Price
	order.Amount = response.Amount
	if response.DealAmount > 0 {
		order.AvgPrice = response.AvgPrice
		order.DealAmount = response.DealAmount
	}
	return resp, nil
}

func (future *Future) GetPairFlow(pair Pair) ([]*FutureAccountItem, []byte, error) {

	var params = url.Values{}
	if err := future.buildParamsSigned(&params); err != nil {
		return nil, nil, err
	}

	var responses = make([]*struct {
		Symbol     string  `json:"symbol"`
		IncomeType string  `json:"incomeType"`
		Income     float64 `json:"income,string"`
		Asset      string  `json:"asset"`
		Info       string  `json:"info"`
		Time       int64   `json:"time"`
	}, 0)

	var resp, err = future.DoRequest(
		http.MethodGet,
		FUTURE_CM_ENDPOINT,
		FUTURE_INCOME_URI+params.Encode(),
		"",
		&responses,
	)
	if err != nil {
		return nil, resp, err
	}

	var items = make([]*FutureAccountItem, 0)
	for i := len(responses) - 1; i >= 0; i-- {
		var r = responses[i]
		if r.Symbol == "" || strings.Index(r.Symbol, "_PERP") > 0 {
			continue
		}

		// 不是这个pair的滤掉
		var symbolFilter = pair.ToSymbol("", true) + "_"
		if strings.Index(r.Symbol, symbolFilter) < 0 {
			continue
		}
		var dateTime = time.Unix(r.Time/1000, 0).In(future.config.Location).Format(GO_BIRTHDAY)
		var fai = &FutureAccountItem{
			Pair:           pair,
			Exchange:       BINANCE,
			ContractName:   pair.ToSymbol("-", true) + "-" + strings.Split(r.Symbol, "_")[1],
			Subject:        future.transferSubject(r.Income, r.IncomeType),
			SettleMode:     SETTLE_MODE_BASIS,
			SettleCurrency: NewCurrency(r.Asset, ""),
			Amount:         r.Income,
			Timestamp:      r.Time,
			DateTime:       dateTime,
			Info:           r.Info,
		}
		items = append(items, fai)
	}

	return items, resp, nil
}

func (future *Future) KeepAlive() {
	nowTimestamp := time.Now().Unix() * 1000
	// last timestamp in 5s, no need to keep alive
	if (nowTimestamp - future.LastTimestamp) < 5*1000 {
		return
	}

	_, _ = future.DoRequest(http.MethodGet, FUTURE_CM_ENDPOINT, FUTURE_KEEP_ALIVE_URI, "", nil)
}

func (future *Future) DoRequest(httpMethod, endPoint, uri, reqBody string, response interface{}) ([]byte, error) {
	resp, err := NewHttpRequest(
		future.config.HttpClient,
		httpMethod,
		endPoint+uri,
		reqBody,
		map[string]string{
			"X-MBX-APIKEY": future.config.ApiKey,
		},
	)
	if err != nil {
		return nil, err
	} else {
		nowTimestamp := time.Now().Unix() * 1000
		if future.LastTimestamp < nowTimestamp {
			future.LastTimestamp = nowTimestamp
		}
		return resp, json.Unmarshal(resp, &response)
	}
}

// get the future contract info.
func (future *Future) getFutureContract(pair Pair, contractType string) (*FutureContract, error) {
	future.Locker.Lock()
	defer future.Locker.Unlock()

	var now = time.Now()
	if now.After(future.nextUpdateContractTime) {
		_, err := future.updateFutureContracts()
		//重试三次
		for i := 0; err != nil && i < 3; i++ {
			time.Sleep(time.Second)
			_, err = future.updateFutureContracts()
		}
		if err != nil {
			return nil, err
		}
	}

	currencies := strings.Split(pair.ToSymbol("_", false), "_")
	contractTypeItem := fmt.Sprintf(
		"%s,%s,%s",
		currencies[0],
		currencies[1],
		contractType,
	)

	cf, exist := future.Contracts.ContractTypeKV[contractTypeItem]
	if !exist {
		return nil, errors.New("Can not find the contract by contract_type. ")
	}
	return cf, nil
}

// update the future contracts info.
func (future *Future) updateFutureContracts() ([]byte, error) {

	var response = struct {
		Symbols []struct {
			Symbol      string `json:"symbol"`
			Pair        string `json:"pair"`
			BaseAsset   string `json:"baseAsset"`
			QuoteAsset  string `json:"quoteAsset"`
			MarginAsset string `json:"marginAsset"`

			ContractType      string  `json:"contractType"`
			DeliveryDate      int64   `json:"deliveryDate"`
			OnboardDate       int64   `json:"onboardDate"`
			ContractStatus    string  `json:"contractStatus"`
			ContractSize      float64 `json:"contractSize"`
			PricePrecision    int64   `json:"pricePrecision"`
			QuantityPrecision int64   `json:"quantityPrecision"`

			Filters []map[string]interface{} `json:"filters"`
		} `json:"symbols"`
		ServerTime int64 `json:"serverTime"`
	}{}

	var resp, err = future.DoRequest(
		http.MethodGet,
		FUTURE_CM_ENDPOINT,
		FUTURE_EXCHANGE_INFO_URI,
		"",
		&response,
	)
	if err != nil {
		return nil, err
	}

	var contracts = FutureContracts{
		ContractTypeKV: make(map[string]*FutureContract, 0),
		ContractNameKV: make(map[string]*FutureContract, 0),
		DueTimestampKV: make(map[string]*FutureContract, 0),
	}

	for _, item := range response.Symbols {
		// it is not future , it's swap in this project.
		if item.ContractType == "PERPETUAL" {
			continue
		}

		if item.ContractType != "CURRENT_QUARTER" && item.ContractType != "NEXT_QUARTER" {
			continue
		}

		var contractType = ""
		if item.ContractType == "CURRENT_QUARTER" {
			contractType = QUARTER_CONTRACT
		} else if item.ContractType == "NEXT_QUARTER" {
			contractType = NEXT_QUARTER_CONTRACT
		} else {
			continue
		}

		settleMode := SETTLE_MODE_BASIS
		if item.MarginAsset == item.QuoteAsset {
			settleMode = SETTLE_MODE_COUNTER
		}

		var priceMaxScale, priceMinScale = float64(1.2), float64(0.8)
		var tickSize = float64(-1)
		for _, filter := range item.Filters {
			if value, ok := filter["filterType"].(string); ok && value == "PERCENT_PRICE" {
				priceMaxScale = ToFloat64(filter["multiplierUp"])
				priceMinScale = ToFloat64(filter["multiplierDown"])
			}

			if value, ok := filter["filterType"].(string); ok && value == "PRICE_FILTER" {
				tickSize = ToFloat64(filter["tickSize"])
			}
		}

		dueTime := time.Unix(item.DeliveryDate/1000, 0).In(future.config.Location)
		openTime := time.Unix(item.OnboardDate/1000, 0).In(future.config.Location)

		pair := Pair{
			Basis:   NewCurrency(item.BaseAsset, ""),
			Counter: NewCurrency(item.QuoteAsset, ""),
		}

		var contractNameInfo = strings.Split(item.Symbol, "_")
		contract := &FutureContract{
			Pair:         pair,
			Symbol:       pair.ToSymbol("_", false),
			Exchange:     BINANCE,
			ContractType: contractType,
			ContractName: pair.ToSymbol("-", true) + "-" + contractNameInfo[1],

			SettleMode:    settleMode,
			OpenTimestamp: openTime.UnixNano() / int64(time.Millisecond),
			OpenDate:      openTime.Format(GO_BIRTHDAY),
			DueTimestamp:  dueTime.UnixNano() / int64(time.Millisecond),
			DueDate:       dueTime.Format(GO_BIRTHDAY),

			UnitAmount:      item.ContractSize,
			TickSize:        tickSize,
			PricePrecision:  item.PricePrecision,
			AmountPrecision: item.QuantityPrecision,

			MaxScalePriceLimit: priceMaxScale,
			MinScalePriceLimit: priceMinScale,
		}

		currencies := strings.Split(contract.Symbol, "_")
		contractTypeItem := fmt.Sprintf("%s,%s,%s", currencies[0], currencies[1], contractType)
		contractNameItem := fmt.Sprintf("%s,%s,%s", currencies[0], currencies[1], contract.ContractName)
		dueTimestampItem := fmt.Sprintf("%s,%s,%d", currencies[0], currencies[1], contract.DueTimestamp)

		contracts.ContractTypeKV[contractTypeItem] = contract
		contracts.ContractNameKV[contractNameItem] = contract
		contracts.DueTimestampKV[dueTimestampItem] = contract
	}

	future.Contracts = contracts
	// setting next update time.
	var nowTime = time.Now().In(future.config.Location)
	var nextUpdateTime = time.Date(
		nowTime.Year(), nowTime.Month(), nowTime.Day(),
		16, 0, 0, 0, future.config.Location,
	)
	if nowTime.Hour() >= 16 {
		nextUpdateTime = nextUpdateTime.AddDate(0, 0, 1)
	}
	future.nextUpdateContractTime = nextUpdateTime
	return resp, nil
}

func (future *Future) getFutureType(side, sidePosition string) FutureType {
	if side == "BUY" && sidePosition == "LONG" {
		return OPEN_LONG
	} else if side == "SELL" && sidePosition == "SHORT" {
		return OPEN_SHORT
	} else if side == "SELL" && sidePosition == "LONG" {
		return LIQUIDATE_LONG
	} else if side == "BUY" && sidePosition == "SHORT" {
		return LIQUIDATE_SHORT
	} else {
		panic("input error, do not use BOTH. ")
	}

}

// return the binance style symbol
func (future *Future) getBNSymbol(contractName string) string {
	var infos = strings.Split(contractName, "-")
	return infos[0] + infos[1] + "_" + infos[2]
}

//var subjectKV = map[string]string{
//	"COMMISSION":   SUBJECT_COMMISSION,
//	"REALIZED_PNL": SUBJECT_SETTLE,
//	"FUNDING_FEE":  SUBJECT_FUNDING_FEE,
//}

func (future *Future) transferSubject(income float64, remoteSubject string) string {
	if remoteSubject == "TRANSFER" {
		if income > 0 {
			return SUBJECT_TRANSFER_IN
		}
		return SUBJECT_TRANSFER_OUT
	}

	if subject, exist := subjectKV[remoteSubject]; exist {
		return subject
	} else {
		return strings.ToLower(remoteSubject)
	}

}

func (future *Future) getCMCandles(
	contract *FutureContract,
	period, size int, since int64,
) ([]*FutureCandle, []byte, error) {

	var endTimestamp = since + int64(size*_INERNAL_KLINE_SECOND_CONVERTER[period])
	if endTimestamp > since+200*24*60*60*1000 {
		endTimestamp = since + 200*24*60*60*1000
	}
	if endTimestamp > time.Now().Unix()*1000 {
		endTimestamp = time.Now().Unix() * 1000
	}

	var pairBN = strings.ToUpper(strings.Replace(contract.Symbol, "_", "", -1))
	params := url.Values{}
	params.Set("pair", pairBN)
	params.Set("contractType", contract.ContractType)
	params.Set("interval", _INERNAL_KLINE_PERIOD_CONVERTER[period])
	params.Set("startTime", fmt.Sprintf("%d", since))
	params.Set("endTime", fmt.Sprintf("%d", endTimestamp))
	params.Set("limit", fmt.Sprintf("%d", size))

	var uri = FUTURE_CM_CANDLE_URI + params.Encode()
	var results = make([][]interface{}, 0)
	resp, err := future.DoRequest(
		http.MethodGet,
		FUTURE_CM_ENDPOINT,
		uri,
		"",
		&results,
	)
	if err != nil {
		return nil, resp, err
	}

	var candles []*FutureCandle = make([]*FutureCandle, 0)
	for _, r := range results {
		var timestamp = ToInt64(r[0])
		var date = time.Unix(timestamp/1000, 0).In(future.config.Location).Format(GO_BIRTHDAY)
		var dueTimestamp = contract.DueTimestamp
		var dueDate = time.Unix(dueTimestamp/1000, 0).In(future.config.Location).Format(GO_BIRTHDAY)

		var c = &FutureCandle{
			Symbol:       contract.Symbol,
			Exchange:     BINANCE,
			Timestamp:    timestamp,
			Date:         date,
			Open:         ToFloat64(r[1]),
			High:         ToFloat64(r[2]),
			Low:          ToFloat64(r[3]),
			Close:        ToFloat64(r[4]),
			Vol:          ToFloat64(r[7]),
			Vol2:         ToFloat64(r[5]),
			Type:         contract.Type,
			DueTimestamp: dueTimestamp,
			DueDate:      dueDate,
		}

		candles = append(candles, c)
	}
	return GetAscFutureCandle(candles), resp, nil
}

func (future *Future) getUMCandles(
	contract *FutureContract,
	period, size int, since int64,
) ([]*FutureCandle, []byte, error) {

	var endTimestamp = since + int64(size*_INERNAL_KLINE_SECOND_CONVERTER[period])
	if endTimestamp > since+200*24*60*60*1000 {
		endTimestamp = since + 200*24*60*60*1000
	}
	if endTimestamp > time.Now().Unix()*1000 {
		endTimestamp = time.Now().Unix() * 1000
	}

	var pairBN = strings.ToUpper(strings.Replace(contract.Symbol, "_", "", -1))
	params := url.Values{}
	params.Set("pair", pairBN)
	params.Set("contractType", contract.ContractType)
	params.Set("interval", _INERNAL_KLINE_PERIOD_CONVERTER[period])
	params.Set("startTime", fmt.Sprintf("%d", since))
	params.Set("endTime", fmt.Sprintf("%d", endTimestamp))
	params.Set("limit", fmt.Sprintf("%d", size))

	var uri = FUTURE_UM_CANDLE_URI + params.Encode()
	var results = make([][]interface{}, 0)
	resp, err := future.DoRequest(
		http.MethodGet,
		FUTURE_UM_ENDPOINT,
		uri,
		"",
		&results,
	)
	if err != nil {
		return nil, resp, err
	}

	var candles = make([]*FutureCandle, 0)
	for _, r := range results {
		var timestamp = ToInt64(r[0])
		var date = time.Unix(timestamp/1000, 0).In(future.config.Location).Format(GO_BIRTHDAY)
		var dueTimestamp = contract.DueTimestamp
		var dueDate = time.Unix(dueTimestamp/1000, 0).In(future.config.Location).Format(GO_BIRTHDAY)

		var c = &FutureCandle{
			Symbol:       contract.Symbol,
			Exchange:     BINANCE,
			Timestamp:    timestamp,
			Date:         date,
			Open:         ToFloat64(r[1]),
			High:         ToFloat64(r[2]),
			Low:          ToFloat64(r[3]),
			Close:        ToFloat64(r[4]),
			Vol:          ToFloat64(r[5]),
			Vol2:         ToFloat64(r[7]),
			Type:         contract.Type,
			DueTimestamp: dueTimestamp,
			DueDate:      dueDate,
		}

		candles = append(candles, c)
	}
	return GetAscFutureCandle(candles), resp, nil
}
