package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/adityadeshlahre/probo-v1/shared/redis"
)

const DefaultContextTimeout = 30

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

var Orders []Order
var Users []User
var Balances []Balance
var Transections []Transection
var Markets []Market

var transectionCounter int = 1

type IncomingMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func main() {
	redis.InitRedis()
	client := redis.GetRedisClient()
	ctx := context.Background()
	for {
		res, err := client.BRPop(ctx, 0, "db-actions").Result()
		if err != nil {
			log.Println("Error popping from queue:", err)
			continue
		}
		message := []byte(res[1])
		err = handleIncomingMessages(message)
		if err != nil {
			log.Println("Error handling message:", err)
		}
	}
}

func handleIncomingMessages(message []byte) error {
	var msg IncomingMessage
	err := json.Unmarshal(message, &msg)
	if err != nil {
		return err
	}
	switch msg.Type {
	case "ORDER":
		var order Order
		err = json.Unmarshal(msg.Data, &order)
		if err != nil {
			return err
		}
		return createOrUpdateOrder(order)
	case "MARKET":
		var market Market
		err = json.Unmarshal(msg.Data, &market)
		if err != nil {
			return err
		}
		return createOrUpdateMarket(market)
	case "USER":
		var user User
		err = json.Unmarshal(msg.Data, &user)
		if err != nil {
			return err
		}
		return createOrUpdateUser(user)
	case "BALANCE":
		var balance Balance
		err = json.Unmarshal(msg.Data, &balance)
		if err != nil {
			return err
		}
		return createOrUpdateBalance(balance)
	case "STOCK":
		var user User
		err = json.Unmarshal(msg.Data, &user)
		if err != nil {
			return err
		}
		return createOrUpdateUserStock(user)
	case "TRANSECTION":
		var transection Transection
		err = json.Unmarshal(msg.Data, &transection)
		if err != nil {
			return err
		}
		return createTransection(transection)
	default:
		return fmt.Errorf("unknown type: %s", msg.Type)
	}
}

func createTransection(data Transection) error {
	data.Id = fmt.Sprintf("transection%d", transectionCounter)
	transectionCounter++
	data.CreatedAt = time.Now().Format(time.RFC3339)
	data.UpdatedAt = data.CreatedAt
	Transections = append(Transections, data)
	return nil
}

func createOrUpdateOrder(data Order) error {
	for i := range Orders {
		if Orders[i].Id == data.Id {
			Orders[i].FilledQty += data.FilledQty
			if data.Status != "" {
				Orders[i].Status = data.Status
			}
			Orders[i].UpdatedAt = time.Now().Format(time.RFC3339)
			return nil
		}
	}
	data.CreatedAt = time.Now().Format(time.RFC3339)
	data.UpdatedAt = data.CreatedAt
	Orders = append(Orders, data)
	return nil
}

func createOrUpdateBalance(data Balance) error {
	for i := range Balances {
		if Balances[i].UserId == data.UserId {
			Balances[i].Balance = data.Balance
			Balances[i].Locked = data.Locked
			return nil
		}
	}
	data.Id = data.UserId
	Balances = append(Balances, data)
	return nil
}

func createOrUpdateUser(data User) error {
	for i := range Users {
		if Users[i].Id == data.Id {
			Users[i] = data
			return nil
		}
	}
	Users = append(Users, data)
	return nil
}

func createOrUpdateUserStock(data User) error {
	for i := range Users {
		if Users[i].Id == data.Id {
			Users[i].Stock = data.Stock
			return nil
		}
	}
	Users = append(Users, data)
	return nil
}

func createOrUpdateMarket(data Market) error {
	for i := range Markets {
		if Markets[i].Id == data.Id {
			Markets[i] = data
			return nil
		}
	}
	data.Id = fmt.Sprintf("market%d", len(Markets)+1)
	data.CreatedAt = time.Now().Format(time.RFC3339)
	data.UpdatedAt = data.CreatedAt
	Markets = append(Markets, data)
	return nil
}
