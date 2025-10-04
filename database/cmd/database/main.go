package main

import "encoding/json"

const DefaultContextTimeout = 30

type Balance struct {
	id      string
	userId  string
	locked  float64
	balance float64
}

type User struct {
	id    string
	stock json.RawMessage
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
	id              string
	userId          string
	orderType       orderType
	quantity        float64
	price           float64
	status          OrderStatus
	symbol          string
	symbolStockType string
	createdAt       string
	updatedAt       string
}

type TransectionType string

const (
	SOLD    TransectionType = "SOLD"
	BOUGHT  TransectionType = "BOUGHT"
	DEPOSIT TransectionType = "DEPOSIT"
	CANCLE  TransectionType = "CANCLE"
)

type Transection struct {
	id              string
	makerId         string
	giverId         string
	takerId         string
	transectionType TransectionType
	quantity        float64
	price           float64
	symbol          string
	symbolStockType string
	createdAt       string
	updatedAt       string
}

type Market struct {
	id                string
	symbol            string
	symbolStockType   string
	sourceOfTruth     string
	heading           string
	eventType         string
	repeatEventTime   string
	endEventAfterTime string
	createdAt         string
	updatedAt         string
}

func main() {
	// redisClient :=
}
