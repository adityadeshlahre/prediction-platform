package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	server "github.com/adityadeshlahre/probo-v1/engine/handler"
	sharedRedis "github.com/adityadeshlahre/probo-v1/shared/redis"
	"github.com/adityadeshlahre/probo-v1/shared/types"
	"github.com/redis/go-redis/v9"
)

const DefaultContextTimeout = 30

var Orders []types.Order
var Users []types.User
var Balances []types.Balance
var Transections []types.Transection
var Markets []types.Market
var dbPublishClient *redis.Client
var client *redis.Client

var transectionCounter int = 1

func main() {
	sharedRedis.InitRedis()
	e := server.NewServer()
	go func() {
		if err := e.Start(":8082"); err != nil {
			log.Fatal(err)
		}
	}()
	client = sharedRedis.GetRedisClient()
	databaseClient := sharedRedis.GetRedisClient()
	databasePubsub := databaseClient.Subscribe(context.Background(), "db-actions")
	dbPublishClient = sharedRedis.GetRedisClient()

	go func() {
		for msg := range databasePubsub.Channel() {
			println("Received message in engine:", msg.Payload)
		}
	}()
	ctx := context.Background()
	for {
		res, err := client.BRPop(ctx, 0, "httptoengine").Result()
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
	var msg types.IncomingMessage
	err := json.Unmarshal(message, &msg)
	if err != nil {
		return err
	}
	switch msg.Type {
	case "ORDER":
		var order types.Order
		err = json.Unmarshal(msg.Data, &order)
		if err != nil {
			return err
		}
		err = createOrUpdateOrder(order)
		if err != nil {
			return err
		}
		dbPublishClient.Publish(context.Background(), "db-actions", message).Err()
		return nil
	case "MARKET":
		var market types.Market
		err = json.Unmarshal(msg.Data, &market)
		if err != nil {
			return err
		}
		err = createOrUpdateMarket(market)
		if err != nil {
			return err
		}
		dbPublishClient.Publish(context.Background(), "db-actions", message).Err()
		return nil
	case "USER":
		var user types.User
		err = json.Unmarshal(msg.Data, &user)
		if err != nil {
			return err
		}
		err = createOrUpdateUser(user)
		if err != nil {
			return err
		}
		client.Publish(context.Background(), "server-responses", message).Err()
		return nil
	case "BALANCE":
		var balance types.Balance
		err = json.Unmarshal(msg.Data, &balance)
		if err != nil {
			return err
		}
		err = createOrUpdateBalance(balance)
		if err != nil {
			return err
		}
		dbPublishClient.Publish(context.Background(), "db-actions", message).Err()
		return nil
	case "STOCK":
		var user types.User
		err = json.Unmarshal(msg.Data, &user)
		if err != nil {
			return err
		}
		err = createOrUpdateUserStock(user)
		if err != nil {
			return err
		}
		dbPublishClient.Publish(context.Background(), "db-actions", message).Err()
		return nil
	case "TRANSECTION":
		var transection types.Transection
		err = json.Unmarshal(msg.Data, &transection)
		if err != nil {
			return err
		}
		err = createTransection(transection)
		if err != nil {
			return err
		}
		dbPublishClient.Publish(context.Background(), "db-actions", message).Err()
		return nil
	default:
		log.Println("Unknown message type:", msg.Type)
	}
	return nil
}

func createTransection(data types.Transection) error {
	data.Id = fmt.Sprintf("transection%d", transectionCounter)
	transectionCounter++
	data.CreatedAt = time.Now().Format(time.RFC3339)
	data.UpdatedAt = data.CreatedAt
	Transections = append(Transections, data)
	return nil
}

func createOrUpdateOrder(data types.Order) error {
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

func createOrUpdateBalance(data types.Balance) error {
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

func createOrUpdateUser(data types.User) error {
	for i := range Users {
		if Users[i].Id == data.Id {
			Users[i] = data
			return nil
		}
	}
	Users = append(Users, data)
	return nil
}

func createOrUpdateUserStock(data types.User) error {
	for i := range Users {
		if Users[i].Id == data.Id {
			Users[i].Stock = data.Stock
			return nil
		}
	}
	Users = append(Users, data)
	return nil
}

func createOrUpdateMarket(data types.Market) error {
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
