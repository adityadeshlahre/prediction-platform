package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"time"

	"github.com/adityadeshlahre/probo-v1/engine/balance"
	"github.com/adityadeshlahre/probo-v1/engine/database"
	server "github.com/adityadeshlahre/probo-v1/engine/handler"
	"github.com/adityadeshlahre/probo-v1/engine/market"
	"github.com/adityadeshlahre/probo-v1/engine/orderbook"
	"github.com/adityadeshlahre/probo-v1/engine/trading"
	sharedRedis "github.com/adityadeshlahre/probo-v1/shared/redis"
	types "github.com/adityadeshlahre/probo-v1/shared/types"
	gonanoid "github.com/matoous/go-nanoid/v2"
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

func addMarketMaker(symbol string) {
	// Add market maker
	USDBalances["marketmaker"] = types.USDBalance{Balance: 10000, Locked: 0}
	if _, exists := StockBalances["marketmaker"]; !exists {
		StockBalances["marketmaker"] = make(types.UserStockBalance)
	}
	StockBalances["marketmaker"][symbol] = types.SymbolStockBalance{
		Yes: types.StockPosition{Quantity: 1000, Locked: 0},
		No:  types.StockPosition{Quantity: 1000, Locked: 0},
	}
	// Send marketmaker to database
	marketmakerUser := types.User{Id: "marketmaker"}
	userData, _ := json.Marshal(marketmakerUser)
	userMsg := types.IncomingMessage{Type: types.USER, Data: userData}
	userBytes, _ := json.Marshal(userMsg)
	engineToDatabaseQueueClient.Publish(context.Background(), types.DB_ACTIONS, userBytes)

	balanceData := types.Balance{Id: "marketmaker", UserId: "marketmaker", Balance: 10000, Locked: 0}
	balanceBytes, _ := json.Marshal(balanceData)
	balanceMsg := types.IncomingMessage{Type: types.BALANCE, Data: balanceBytes}
	balanceMsgBytes, _ := json.Marshal(balanceMsg)
	engineToDatabaseQueueClient.Publish(context.Background(), types.DB_ACTIONS, balanceMsgBytes)

	stockData := types.User{Id: "marketmaker", Stock: json.RawMessage(fmt.Sprintf(`{"%s":{"yes":{"quantity":1000,"locked":0},"no":{"quantity":1000,"locked":0}}}`, symbol))}
	stockBytes, _ := json.Marshal(stockData)
	stockMsg := types.IncomingMessage{Type: types.STOCK, Data: stockBytes}
	stockMsgBytes, _ := json.Marshal(stockMsg)
	engineToDatabaseQueueClient.Publish(context.Background(), types.DB_ACTIONS, stockMsgBytes)

	// Add market maker orders
	orderId1, _ := gonanoid.Generate("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", 21)
	order1 := types.Order{
		Id:              orderId1,
		UserId:          "marketmaker",
		OrderType:       types.SELL,
		Symbol:          symbol,
		SymbolStockType: "yes",
		Price:           6.0,
		Quantity:        100,
		FilledQty:       0,
		Status:          types.PENDING,
		CreatedAt:       time.Now().Format(time.RFC3339),
		UpdatedAt:       time.Now().Format(time.RFC3339),
	}
	orderData1, _ := json.Marshal(order1)
	orderMsg1 := types.IncomingMessage{Type: "ORDER", Data: orderData1}
	orderMsgBytes1, _ := json.Marshal(orderMsg1)
	engineToDatabaseQueueClient.Publish(context.Background(), types.DB_ACTIONS, orderMsgBytes1)

	orderId2, _ := gonanoid.Generate("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", 21)
	order2 := types.Order{
		Id:              orderId2,
		UserId:          "marketmaker",
		OrderType:       types.BUY,
		Symbol:          symbol,
		SymbolStockType: "no",
		Price:           4.0,
		Quantity:        100,
		FilledQty:       0,
		Status:          types.PENDING,
		CreatedAt:       time.Now().Format(time.RFC3339),
		UpdatedAt:       time.Now().Format(time.RFC3339),
	}
	orderData2, _ := json.Marshal(order2)
	orderMsg2 := types.IncomingMessage{Type: "ORDER", Data: orderData2}
	orderMsgBytes2, _ := json.Marshal(orderMsg2)
	engineToDatabaseQueueClient.Publish(context.Background(), types.DB_ACTIONS, orderMsgBytes2)

	// Update order book
	priceMap := OrderBook[symbol].Yes
	if _, exists := priceMap[6.0]; !exists {
		priceMap[6.0] = types.PriceLevel{Total: 0, Orders: make(map[string]types.OrderBookEntry)}
	}
	priceLevel := priceMap[6.0]
	priceLevel.Total += 100
	priceLevel.Orders[orderId1] = types.OrderBookEntry{UserId: "marketmaker", Quantity: 100, Type: "regular"}
	priceMap[6.0] = priceLevel

	priceMapNo := OrderBook[symbol].No
	if _, exists := priceMapNo[4.0]; !exists {
		priceMapNo[4.0] = types.PriceLevel{Total: 0, Orders: make(map[string]types.OrderBookEntry)}
	}
	priceLevelNo := priceMapNo[4.0]
	priceLevelNo.Total += 100
	priceLevelNo.Orders[orderId2] = types.OrderBookEntry{UserId: "marketmaker", Quantity: 100, Type: "reverted"}
	priceMapNo[4.0] = priceLevelNo

	OrderBook[symbol] = types.SymbolOrderBook{Yes: priceMap, No: priceMapNo}
}

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

	databaseActionsClient := sharedRedis.GetRedisClient()
	databasePubsub := databaseActionsClient.Subscribe(context.Background(), types.DB_ACTIONS)

	go func() {
		for msg := range databasePubsub.Channel() {
			println("Received message in engine:", msg.Payload)
		}
	}()

	engineResponseSubscriber = sharedRedis.GetRedisClient()
	engineResponsePubsub := engineResponseSubscriber.Subscribe(context.Background(), types.ENGINE_RESPONSES)

	go func() {
		for msg := range engineResponsePubsub.Channel() {
			var resp types.IncomingMessage
			if err := json.Unmarshal([]byte(msg.Payload), &resp); err == nil {
				var key string
				switch resp.Type {
				case types.USER:
					var user types.User
					if err := json.Unmarshal(resp.Data, &user); err == nil {
						key = user.Id
					}
				case types.BALANCE:
					var balance types.Balance
					if err := json.Unmarshal(resp.Data, &balance); err == nil {
						key = balance.Id
						// Update USDBalances
						USDBalances[balance.UserId] = types.USDBalance{
							Balance: balance.Balance,
							Locked:  balance.Locked,
						}
					}
				case types.ORDER:
					var order types.Order
					if err := json.Unmarshal(resp.Data, &order); err == nil {
						key = order.Id
					}
				case types.MARKET:
					var market types.Market
					if err := json.Unmarshal(resp.Data, &market); err == nil {
						key = market.Id
					}
				case types.TRANSECTION:
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
		res, err := engineFromServerQueueClient.BRPop(ctx, 0, types.HTTP_TO_ENGINE).Result()
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
	case types.ORDER:
		var order types.Order
		err = json.Unmarshal(msg.Data, &order)
		if err != nil {
			return err
		}
		err = database.CreateOrUpdateOrder(order)
		if err != nil {
			return err
		}
		engineToDatabaseQueueClient.Publish(context.Background(), types.DB_ACTIONS, message).Err()
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
			Type: types.ONRAMP_USD,
			Data: json.RawMessage(fmt.Sprintf(`{"userId":"%s","amount":%f,"status":"success"}`, onRampReq.UserId, onRampReq.Amount)),
		}
		responseBytes, _ := json.Marshal(responseMsg)
		engineToServerPubSubClient.LPush(context.Background(), "SERVER_RESPONSES_QUEUE", responseBytes).Err()
		return nil

	case types.MARKET:
		var market types.Market
		err = json.Unmarshal(msg.Data, &market)
		if err != nil {
			return err
		}
		err = database.CreateOrUpdateMarket(market)
		if err != nil {
			return err
		}
		engineToDatabaseQueueClient.Publish(context.Background(), types.DB_ACTIONS, message).Err()
		return nil

	case types.USER:
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
		newBalance := types.Balance{Id: balanceId, UserId: user.Id, Balance: 10000, Locked: 0}
		err = database.CreateOrUpdateBalance(newBalance)
		if err != nil {
			return err
		}
		// Update in-memory USDBalances immediately
		USDBalances[user.Id] = types.USDBalance{Balance: 10000, Locked: 0}
		engineToDatabaseQueueClient.Publish(context.Background(), types.DB_ACTIONS, message).Err()
		engineToServerPubSubClient.LPush(context.Background(), "SERVER_RESPONSES_QUEUE", message).Err()
		return nil

	case types.BALANCE:
		var balance types.Balance
		err = json.Unmarshal(msg.Data, &balance)
		if err != nil {
			return err
		}
		err = database.CreateOrUpdateBalance(balance)
		if err != nil {
			return err
		}
		engineToDatabaseQueueClient.Publish(context.Background(), types.DB_ACTIONS, message).Err()
		return nil

	case types.STOCK:
		var user types.User
		err = json.Unmarshal(msg.Data, &user)
		if err != nil {
			return err
		}
		err = database.CreateOrUpdateUserStock(user)
		if err != nil {
			return err
		}
		engineToDatabaseQueueClient.Publish(context.Background(), types.DB_ACTIONS, message).Err()
		return nil

	case types.TRANSECTION:
		var transection types.Transection
		err = json.Unmarshal(msg.Data, &transection)
		if err != nil {
			return err
		}
		err = database.CreateTransection(transection)
		if err != nil {
			return err
		}
		engineToDatabaseQueueClient.Publish(context.Background(), types.DB_ACTIONS, message).Err()
		return nil

	case types.GET_ORDER_BOOK:
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
		orderBookData, err := json.Marshal(orderBook)
		if err != nil {
			return err
		}
		responseData := fmt.Sprintf(`{"symbol":"%s","orderBook":%s}`, req.Symbol, string(orderBookData))
		responseMsg := types.IncomingMessage{
			Type: types.GET_ORDER_BOOK,
			Data: json.RawMessage(responseData),
		}
		responseBytes, err := json.Marshal(responseMsg)
		if err != nil {
			return err
		}
		err = engineToServerPubSubClient.LPush(context.Background(), types.SERVER_RESPONSES_QUEUE, responseBytes).Err()
		if err != nil {
			return err
		}
		return nil

	case types.GET_ALL_ORDER_BOOK:
		allOrderBooks, err := orderbook.GetAllOrderBooks()
		if err != nil {
			return err
		}
		orderBooksData, err := json.Marshal(allOrderBooks)
		if err != nil {
			return err
		}
		responseMsg := types.IncomingMessage{
			Type: types.GET_ALL_ORDER_BOOK,
			Data: json.RawMessage(string(orderBooksData)),
		}
		responseBytes, err := json.Marshal(responseMsg)
		if err != nil {
			return err
		}
		err = engineToServerPubSubClient.LPush(context.Background(), types.SERVER_RESPONSES_QUEUE, responseBytes).Err()
		if err != nil {
			return err
		}
		return nil

	case types.CANCLE_ORDER:
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
			Type: types.CANCLE_ORDER,
			Data: json.RawMessage(fmt.Sprintf(`{"orderId":"%s","status":"cancelled"}`, cancelReq.OrderId)),
		}
		responseBytes, _ := json.Marshal(responseMsg)
		engineToServerPubSubClient.LPush(context.Background(), "SERVER_RESPONSES_QUEUE", responseBytes).Err()
		return nil

	case types.END_MARKET:
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
			Type: types.END_MARKET,
			Data: json.RawMessage(fmt.Sprintf(`{"marketId":"%s","status":"ended","winner":"%s"}`, endReq.MarketId, endReq.WinningStock)),
		}
		responseBytes, _ := json.Marshal(responseMsg)
		engineToServerPubSubClient.LPush(context.Background(), "SERVER_RESPONSES_QUEUE", responseBytes).Err()
		return nil

	case types.BUY_ORDER:
		var orderProps types.OrderProps
		err = json.Unmarshal(msg.Data, &orderProps)
		if err != nil {
			return err
		}
		result, err := trading.PlaceBuyOrder(orderProps)
		if err != nil {
			// Send error response
			responseMsg := types.IncomingMessage{
				Type: types.BUY_ORDER,
				Data: json.RawMessage(fmt.Sprintf(`{"error":"%s","userId":"%s"}`, err.Error(), orderProps.UserId)),
			}
			responseBytes, _ := json.Marshal(responseMsg)
			engineToServerPubSubClient.LPush(context.Background(), "SERVER_RESPONSES_QUEUE", responseBytes).Err()
			return err
		}
		// Send success response
		result["userId"] = orderProps.UserId
		resultData, _ := json.Marshal(result)
		responseMsg := types.IncomingMessage{
			Type: types.BUY_ORDER,
			Data: resultData,
		}
		responseBytes, _ := json.Marshal(responseMsg)
		engineToServerPubSubClient.LPush(context.Background(), "SERVER_RESPONSES_QUEUE", responseBytes).Err()
		return nil

	case types.SELL_ORDER:
		var orderProps types.OrderProps
		err = json.Unmarshal(msg.Data, &orderProps)
		if err != nil {
			return err
		}
		result, err := trading.PlaceSellOrder(orderProps)
		if err != nil {
			// Send error response
			responseMsg := types.IncomingMessage{
				Type: types.SELL_ORDER,
				Data: json.RawMessage(fmt.Sprintf(`{"error":"%s","userId":"%s"}`, err.Error(), orderProps.UserId)),
			}
			responseBytes, _ := json.Marshal(responseMsg)
			engineToServerPubSubClient.LPush(context.Background(), "SERVER_RESPONSES_QUEUE", responseBytes).Err()
			return err
		}
		// Send success response
		result["userId"] = orderProps.UserId
		resultData, _ := json.Marshal(result)
		responseMsg := types.IncomingMessage{
			Type: types.SELL_ORDER,
			Data: resultData,
		}
		responseBytes, _ := json.Marshal(responseMsg)
		engineToServerPubSubClient.LPush(context.Background(), "SERVER_RESPONSES_QUEUE", responseBytes).Err()
		return nil

	case types.CREATE_MARKET:
		var createReq types.CreateMarket
		err = json.Unmarshal(msg.Data, &createReq)
		if err != nil {
			return err
		}
		err = market.CreateMarket(createReq)
		if err != nil {
			return err
		}
		// Add market maker
		addMarketMaker(createReq.Symbol)

		// Send success response
		responseMsg := types.IncomingMessage{
			Type: types.CREATE_MARKET,
			Data: json.RawMessage(fmt.Sprintf(`{"%s":{"status":"created"}}`, createReq.Symbol)),
		}
		responseBytes, _ := json.Marshal(responseMsg)
		engineToServerPubSubClient.LPush(context.Background(), "SERVER_RESPONSES_QUEUE", responseBytes).Err()
		return nil

	default:
		log.Println("Unknown message type:", msg.Type)
	}
	return nil
}
