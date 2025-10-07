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
	ONRAMP_USD     ActionType = "ONRAMP_USD"
	CANCLE_ORDER   ActionType = "CANCLE_ORDER"
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
	RepeatEventTime int64  `json:"repeatEventTime"`
}

// OrderBook Types (equivalent to TypeScript interfaces)
type OrderDetails struct {
	UserId   string  `json:"userId"`
	Quantity float64 `json:"quantity"`
	Type     string  `json:"type"` // "reverted" | "regular"
}

type OrderBookOrders map[string]OrderDetails

type OrderBookPerPrice struct {
	Total  float64         `json:"total"`
	Orders OrderBookOrders `json:"orders"`
}

type OrderBookPrices map[float64]OrderBookPerPrice

type OrderBookPerStock struct {
	Yes OrderBookPrices `json:"yes"`
	No  OrderBookPrices `json:"no"`
}

type OrderBook map[string]OrderBookPerStock

// OnRampProps for USD deposits
type OnRampProps struct {
	UserId string  `json:"userId"`
	Amount float64 `json:"amount"`
}

// CancelOrderProps for order cancellation
type CancelOrderProps struct {
	UserId      string  `json:"userId"`
	StockSymbol string  `json:"stockSymbol"`
	OrderId     string  `json:"orderId"`
	StockType   string  `json:"stockType"` // "yes" | "no"
	Price       float64 `json:"price"`
}

// OrderProps for placing orders
type OrderProps struct {
	UserId      string  `json:"userId"`
	StockSymbol string  `json:"stockSymbol"`
	Quantity    float64 `json:"quantity"`
	Price       float64 `json:"price"`
	StockType   string  `json:"stockType"` // "yes" | "no"
}

// Enhanced Market with status tracking
type MarketStatus string

const (
	MarketActive    MarketStatus = "Active"
	MarketCompleted MarketStatus = "COMPLETED"
)

type MarketType string

const (
	MarketAutomatic MarketType = "automatic"
	MarketManual    MarketType = "manual"
)

type EnhancedMarket struct {
	StockSymbol string       `json:"stockSymbol"`
	Price       float64      `json:"price"`
	Heading     string       `json:"heading"`
	EventType   string       `json:"eventType"`
	Type        MarketType   `json:"type"`
	Status      MarketStatus `json:"status"`
}

type Markets map[string]EnhancedMarket

type USDBalances map[string]USDBalance

type USDBalance struct {
	Balance float64 `json:"balance"`
	Locked  float64 `json:"locked"`
}

type StockBalances map[string]UserStockBalance

type UserStockBalance map[string]SymbolStockBalance

type SymbolStockBalance struct {
	Yes StockPosition `json:"yes"`
	No  StockPosition `json:"no"`
}

type StockPosition struct {
	Quantity float64 `json:"quantity"`
	Locked   float64 `json:"locked"`
}

type YesNoOrderBook map[string]SymbolOrderBook

type SymbolOrderBook struct {
	Yes PriceOrderBook `json:"yes"`
	No  PriceOrderBook `json:"no"`
}

type PriceOrderBook map[float64]PriceLevel

type PriceLevel struct {
	Total  float64                   `json:"total"`
	Orders map[string]OrderBookEntry `json:"orders"`
}

type OrderBookEntry struct {
	UserId   string  `json:"userId"`
	Quantity float64 `json:"quantity"`
	Type     string  `json:"type"` // "reverted" | "regular"
}
