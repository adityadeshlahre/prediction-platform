package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/adityadeshlahre/probo-v1/engine/balance"
	"github.com/adityadeshlahre/probo-v1/engine/database"
	"github.com/adityadeshlahre/probo-v1/engine/market"
	"github.com/adityadeshlahre/probo-v1/engine/orderbook"
	server "github.com/adityadeshlahre/probo-v1/engine/handler"
	sharedRedis "github.com/adityadeshlahre/probo-v1/shared/redis"
	"github.com/adityadeshlahre/probo-v1/engine/trading"
	types "github.com/adityadeshlahre/probo-v1/shared/types"
	"github.com/redis/go-redis/v9"
)

var Orders []types.Order
var Users []types.User
var Balances []types.Balance
var Transections []types.Transection
var Markets []types.Market

var USDBalances types.USDBalances = make(types.USDBalances)
var StockBalances types.StockBalances = make(types.StockBalances)
var OrderBook types.YesNoOrderBook = make(types.YesNoOrderBook)
var MarketsMap types.Markets = make(types.Markets)

var engineToDatabaseQueueClient *redis.Client
var engineFromServerQueueClient *redis.Client
var engineToServerPubSubClient *redis.Client
var engineResponseSubscriber *redis.Client

var transectionCounter int = 0
var EngineAwaitsForResponseMap = make(map[string]chan string)

func main() {
	sharedRedis.InitRedis()
	e := server.NewServer()
	go func() {
		if err := e.Start(":8082"); err != nil {
			log.Fatal(err)
		}
	}()

	engineToDatabaseQueueClient = sharedRedis.GetRedisClient()
	engineFromServerQueueClient = sharedRedis.GetRedisClient()
	engineToServerPubSubClient = sharedRedis.GetRedisClient()

	// Initialize packages with data structures and clients
	trading.SetClients(engineToDatabaseQueueClient, engineToServerPubSubClient)
	trading.SetDataStructures(USDBalances, StockBalances, OrderBook)

	market.SetClients(engineToDatabaseQueueClient, engineToServerPubSubClient)
	market.SetDataStructures(USDBalances, StockBalances, OrderBook, MarketsMap, &Orders, &Transections)

	balance.SetClients(engineToDatabaseQueueClient)
	balance.SetDataStructures(USDBalances, StockBalances, &Balances, &Transections)

	orderbook.SetDataStructures(OrderBook)
	database.SetDataStructures(&Orders, &Users, &Balances, &Transections, &Markets, &transectionCounter)

	databaseClient := sharedRedis.GetRedisClient()
	databasePubsub := databaseClient.Subscribe(context.Background(), "DB_ACTIONS")

	go func() {
		for msg := range databasePubsub.Channel() {
			println("Received message in engine:", msg.Payload)
		}
	}()

	engineResponseSubscriber = sharedRedis.GetRedisClient()
	engineResponsePubsub := engineResponseSubscriber.Subscribe(context.Background(), "ENGINE_RESPONSES")

	go func() {
		for msg := range engineResponsePubsub.Channel() {
			var resp types.IncomingMessage
			if err := json.Unmarshal([]byte(msg.Payload), &resp); err == nil {
				var key string
				switch resp.Type {
				case "USER":
					var user types.User
					if err := json.Unmarshal(resp.Data, &user); err == nil {
						key = user.Id
					}
				case "BALANCE":
					var balance types.Balance
					if err := json.Unmarshal(resp.Data, &balance); err == nil {
						key = balance.Id
					}
				case "ORDER":
					var order types.Order
					if err := json.Unmarshal(resp.Data, &order); err == nil {
						key = order.Id
					}
				case "MARKET":
					var market types.Market
					if err := json.Unmarshal(resp.Data, &market); err == nil {
						key = market.Id
					}
				case "TRANSECTION":
					var transection types.Transection
					if err := json.Unmarshal(resp.Data, &transection); err == nil {
						key = transection.Id
					}
				case "STOCK":
					var user types.User
					if err := json.Unmarshal(resp.Data, &user); err == nil {
						key = user.Id
					}
				}
				if key != "" {
					if ch, ok := EngineAwaitsForResponseMap[key]; ok {
						ch <- msg.Payload
						delete(EngineAwaitsForResponseMap, key)
					}
				}
			}
			println("Received response in engine:", msg.Payload)
		}
	}()

	ctx := context.Background()
	for {
		res, err := engineFromServerQueueClient.BRPop(ctx, 0, "HTTP_TO_ENGINE").Result()
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
		err = database.CreateOrUpdateOrder(order)
		if err != nil {
			return err
		}
		engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", message).Err()
		return nil

	case string(types.ONRAMP_USD):
		var onRampReq types.OnRampProps
		err = json.Unmarshal(msg.Data, &onRampReq)
		if err != nil {
			return err
		}
		err = balance.OnRampUSD(onRampReq.UserId, onRampReq.Amount)
		if err != nil {
			return err
		}
		// Send success response
		responseMsg := types.IncomingMessage{
			Type: string(types.ONRAMP_USD),
			Data: json.RawMessage(fmt.Sprintf(`{"userId":"%s","amount":%f,"status":"success"}`, onRampReq.UserId, onRampReq.Amount)),
		}
		responseBytes, _ := json.Marshal(responseMsg)
		engineToServerPubSubClient.Publish(context.Background(), "SERVER_RESPONSES", responseBytes).Err()
		return nil

	case "MARKET":
		var market types.Market
		err = json.Unmarshal(msg.Data, &market)
		if err != nil {
			return err
		}
		err = database.CreateOrUpdateMarket(market)
		if err != nil {
			return err
		}
		engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", message).Err()
		return nil

	case "USER":
		var user types.User
		err = json.Unmarshal(msg.Data, &user)
		if err != nil {
			return err
		}
		err = database.CreateOrUpdateUser(user)
		if err != nil {
			return err
		}
		// Create balance for new user
		balanceId := user.Id // Use user ID as balance ID for simplicity
		newBalance := types.Balance{Id: balanceId, UserId: user.Id, Balance: 1000, Locked: 0}
		err = database.CreateOrUpdateBalance(newBalance)
		if err != nil {
			return err
		}
		engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", message).Err()
		engineToServerPubSubClient.Publish(context.Background(), "SERVER_RESPONSES", message).Err()
		return nil

	case "BALANCE":
		var balance types.Balance
		err = json.Unmarshal(msg.Data, &balance)
		if err != nil {
			return err
		}
		err = database.CreateOrUpdateBalance(balance)
		if err != nil {
			return err
		}
		engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", message).Err()
		return nil

	case "STOCK":
		var user types.User
		err = json.Unmarshal(msg.Data, &user)
		if err != nil {
			return err
		}
		err = database.CreateOrUpdateUserStock(user)
		if err != nil {
			return err
		}
		engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", message).Err()
		return nil

	case "TRANSECTION":
		var transection types.Transection
		err = json.Unmarshal(msg.Data, &transection)
		if err != nil {
			return err
		}
		err = database.CreateTransection(transection)
		if err != nil {
			return err
		}
		engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", message).Err()
		return nil

	case string(types.GET_ORDER_BOOK):
		var req struct {
			Symbol string `json:"symbol"`
		}
		err = json.Unmarshal(msg.Data, &req)
		if err != nil {
			return err
		}
		orderBook, err := orderbook.GetOrderBook(req.Symbol)
		if err != nil {
			return err
		}
		orderBookData, _ := json.Marshal(orderBook)
		responseMsg := types.IncomingMessage{
			Type: string(types.GET_ORDER_BOOK),
			Data: orderBookData,
		}
		responseBytes, _ := json.Marshal(responseMsg)
		engineToServerPubSubClient.Publish(context.Background(), "SERVER_RESPONSES", responseBytes).Err()
		return nil

	case string(types.CANCLE_ORDER):
		var cancelReq types.CancelOrderProps
		err = json.Unmarshal(msg.Data, &cancelReq)
		if err != nil {
			return err
		}
		err = trading.CancelOrder(cancelReq)
		if err != nil {
			return err
		}
		// Send success response
		responseMsg := types.IncomingMessage{
			Type: string(types.CANCLE_ORDER),
			Data: json.RawMessage(fmt.Sprintf(`{"orderId":"%s","status":"cancelled"}`, cancelReq.OrderId)),
		}
		responseBytes, _ := json.Marshal(responseMsg)
		engineToServerPubSubClient.Publish(context.Background(), "SERVER_RESPONSES", responseBytes).Err()
		return nil

	case string(types.END_MARKET):
		var endReq struct {
			StockSymbol  string `json:"stockSymbol"`
			MarketId     string `json:"marketId"`
			WinningStock string `json:"winningStock"`
		}
		err = json.Unmarshal(msg.Data, &endReq)
		if err != nil {
			return err
		}
		err = market.EndMarket(endReq.StockSymbol, endReq.WinningStock)
		if err != nil {
			return err
		}
		// Send success response
		responseMsg := types.IncomingMessage{
			Type: string(types.END_MARKET),
			Data: json.RawMessage(fmt.Sprintf(`{"marketId":"%s","status":"ended","winner":"%s"}`, endReq.MarketId, endReq.WinningStock)),
		}
		responseBytes, _ := json.Marshal(responseMsg)
		engineToServerPubSubClient.Publish(context.Background(), "SERVER_RESPONSES", responseBytes).Err()
		return nil

	case string(types.BUY_ORDER):
		var orderProps types.OrderProps
		err = json.Unmarshal(msg.Data, &orderProps)
		if err != nil {
			return err
		}
		result, err := trading.PlaceBuyOrder(orderProps)
		if err != nil {
			// Send error response
			responseMsg := types.IncomingMessage{
				Type: string(types.BUY_ORDER),
				Data: json.RawMessage(fmt.Sprintf(`{"error":"%s"}`, err.Error())),
			}
			responseBytes, _ := json.Marshal(responseMsg)
			engineToServerPubSubClient.Publish(context.Background(), "SERVER_RESPONSES", responseBytes).Err()
			return err
		}
		// Send success response
		resultData, _ := json.Marshal(result)
		responseMsg := types.IncomingMessage{
			Type: string(types.BUY_ORDER),
			Data: resultData,
		}
		responseBytes, _ := json.Marshal(responseMsg)
		engineToServerPubSubClient.Publish(context.Background(), "SERVER_RESPONSES", responseBytes).Err()
		return nil

	case string(types.SELL_ORDER):
		var orderProps types.OrderProps
		err = json.Unmarshal(msg.Data, &orderProps)
		if err != nil {
			return err
		}
		result, err := trading.PlaceSellOrder(orderProps)
		if err != nil {
			// Send error response
			responseMsg := types.IncomingMessage{
				Type: string(types.SELL_ORDER),
				Data: json.RawMessage(fmt.Sprintf(`{"error":"%s"}`, err.Error())),
			}
			responseBytes, _ := json.Marshal(responseMsg)
			engineToServerPubSubClient.Publish(context.Background(), "SERVER_RESPONSES", responseBytes).Err()
			return err
		}
		// Send success response
		resultData, _ := json.Marshal(result)
		responseMsg := types.IncomingMessage{
			Type: string(types.SELL_ORDER),
			Data: resultData,
		}
		responseBytes, _ := json.Marshal(responseMsg)
		engineToServerPubSubClient.Publish(context.Background(), "SERVER_RESPONSES", responseBytes).Err()
		return nil

	case string(types.CREATE_MARKET):
		var createReq types.CreateMarket
		err = json.Unmarshal(msg.Data, &createReq)
		if err != nil {
			return err
		}
		err = market.CreateMarket(createReq)
		if err != nil {
			return err
		}
		// Send success response
		responseMsg := types.IncomingMessage{
			Type: string(types.CREATE_MARKET),
			Data: json.RawMessage(fmt.Sprintf(`{"symbol":"%s","status":"created"}`, createReq.Symbol)),
		}
		responseBytes, _ := json.Marshal(responseMsg)
		engineToServerPubSubClient.Publish(context.Background(), "SERVER_RESPONSES", responseBytes).Err()
		return nil

	default:
		log.Println("Unknown message type:", msg.Type)
	}
	return nil
}
