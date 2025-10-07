package types

import (
	"encoding/json"
)

type IncomingMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type Balance struct {
	Id      string  `json:"id"`
	UserId  string  `json:"userId"`
	Locked  float64 `json:"locked"`
	Balance float64 `json:"balance"`
}

type User struct {
	Id    string          `json:"id"`
	Stock json.RawMessage `json:"stock"`
}

type OrderStatus string

const (
	PENDING   OrderStatus = "PENDING"
	COMPLETED OrderStatus = "COMPLETED"
	CANCELLED OrderStatus = "CANCELLED"
)

type orderType string

const (
	BUY  orderType = "BUY"
	SELL orderType = "SELL"
)

type symbolStockType string

const (
	YES symbolStockType = "YES"
	NO  symbolStockType = "NO"
)

type Order struct {
	Id              string      `json:"id"`
	UserId          string      `json:"userId"`
	OrderType       orderType   `json:"orderType"`
	Quantity        float64     `json:"quantity"`
	FilledQty       float64     `json:"filledQty"`
	Price           float64     `json:"price"`
	Status          OrderStatus `json:"status"`
	Symbol          string      `json:"symbol"`
	SymbolStockType string      `json:"symbolStockType"`
	CreatedAt       string      `json:"createdAt"`
	UpdatedAt       string      `json:"updatedAt"`
}

type TransectionType string

const (
	SOLD    TransectionType = "SOLD"
	BOUGHT  TransectionType = "BOUGHT"
	DEPOSIT TransectionType = "DEPOSIT"
	CANCLE  TransectionType = "CANCLE"
)

type Transection struct {
	Id              string          `json:"id"`
	MakerId         string          `json:"makerId"` // userId
	GiverId         []string        `json:"giverId"` // exchangerID
	TakerId         string          `json:"takerId"` // userId
	TransectionType TransectionType `json:"transectionType"`
	Quantity        float64         `json:"quantity"`
	Price           float64         `json:"price"`
	Symbol          string          `json:"symbol"`
	SymbolStockType string          `json:"symbolStockType"`
	CreatedAt       string          `json:"createdAt"`
	UpdatedAt       string          `json:"updatedAt"`
}

type Market struct {
	Id                string `json:"id"`
	Symbol            string `json:"symbol"`
	SymbolStockType   string `json:"symbolStockType"`
	SourceOfTruth     string `json:"sourceOfTruth"`
	Heading           string `json:"heading"`
	EventType         string `json:"eventType"`
	RepeatEventTime   string `json:"repeatEventTime"`
	EndEventAfterTime string `json:"endEventAfterTime"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

type ActionType string

const (
	SELL_ORDER     ActionType = "SELL_ORDER"
	BUY_ORDER      ActionType = "BUY_ORDER"
	GET_USD        ActionType = "GET_USD"
	GET_STOCKS     ActionType = "GET_STOCKS"
	USER_USD       ActionType = "USER_USD"
	USER_STOCKS    ActionType = "USER_STOCKS"
	GET_ORDER_BOOK ActionType = "GET_ORDER_BOOK"
	GET_MARKET     ActionType = "GET_MARKET"
	CREATE_USER    ActionType = "CREATE_USER"
	CREATE_MARKET  ActionType = "CREATE_MARKET"
	END_MARKET     ActionType = "END_MARKET"
)

type marketType string

const (
	AUTOMATIC marketType = "AUTOMATIC"
	MANUAL    marketType = "MANUAL"
)

type sourceOfTruth string

const (
	AUTOMATIC_TRIGGERS sourceOfTruth = "AUTOMATIC_TRIGGERS"
	MANUAL_TRIGGERS    sourceOfTruth = "MANUAL_TRIGGERS"
)

type CreateMarket struct {
	Symbol          string `json:"symbol"`
	MarketType      string `json:"marketType"`
	EndsIn          int64  `json:"endsIn"`
	SourceOfTruth   string `json:"sourceOfTruth"`
	EndAfterTime    int64  `json:"endAfterTime"`
	Heading         string `json:"heading"`
	EventType       string `json:"eventType"`
	RepeatEventTime int64 `json:"repeatEventTime"`
}
