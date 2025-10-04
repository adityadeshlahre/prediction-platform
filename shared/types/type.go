package types

import "encoding/json"

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
