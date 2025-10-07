package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	server "github.com/adityadeshlahre/probo-v1/engine/handler"
	sharedRedis "github.com/adityadeshlahre/probo-v1/shared/redis"
	types "github.com/adityadeshlahre/probo-v1/shared/types"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/redis/go-redis/v9"
)

const DefaultContextTimeout = 30

var Orders []types.Order
var Users []types.User
var Balances []types.Balance
var Transections []types.Transection
var Markets []types.Market
var OrderBook types.OrderBook = make(types.OrderBook)

var engineToDatabaseQueueClient *redis.Client
var engineFromServerQueueClient *redis.Client

// var engineToDatabasePubSubClient *redis.Client
var engineToServerPubSubClient *redis.Client

// var engineToSocketPubSubClient *redis.Client

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
	// engineToSocketPubSubClient = sharedRedis.GetRedisClient()
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
		err = createOrUpdateOrder(order)
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
		err = onRampUSD(onRampReq.UserId, onRampReq.Amount)
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
		err = createOrUpdateMarket(market)
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
		err = createOrUpdateUser(user)
		if err != nil {
			return err
		}

		err = sendMessageToDatabaseQueueAndAwaitForResponseViaPubSubClient(message, user.Id)
		if err != nil {
			return err
		}

		balanceId, err := gonanoid.New()
		if err != nil {
			return err
		}
		balance := types.Balance{Id: balanceId, UserId: user.Id, Balance: 1000, Locked: 0}
		go createOrUpdateBalance(balance)

		balanceData, err := json.Marshal(balance)
		if err != nil {
			return err
		}
		balanceMsg := types.IncomingMessage{
			Type: "BALANCE",
			Data: balanceData,
		}
		balanceMsgBytes, err := json.Marshal(balanceMsg)
		if err != nil {
			return err
		}
		go engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", balanceMsgBytes).Err()
		engineToServerPubSubClient.Publish(context.Background(), "SERVER_RESPONSES", message).Err()
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
		engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", message).Err()
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
		engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", message).Err()
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
		orderBook, err := getOrderBook(req.Symbol)
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
		err = cancelOrder(cancelReq)
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
		err = endMarket(endReq.MarketId, endReq.StockSymbol, endReq.WinningStock)
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
	default:
		log.Println("Unknown message type:", msg.Type)
	}
	return nil
}

func onRampUSD(userId string, amount float64) error {
	// Find or create balance for user
	found := false
	for i := range Balances {
		if Balances[i].UserId == userId {
			Balances[i].Balance += amount
			found = true
			break
		}
	}

	if !found {
		// Create new balance if user doesn't have one
		balanceId, err := gonanoid.New()
		if err != nil {
			return err
		}
		Balances = append(Balances, types.Balance{
			Id:      balanceId,
			UserId:  userId,
			Balance: amount,
			Locked:  0,
		})
	}

	// Create transaction record
	transectionId, err := gonanoid.New()
	if err != nil {
		return err
	}

	transection := types.Transection{
		Id:              transectionId,
		MakerId:         userId,
		TakerId:         userId,
		TransectionType: types.DEPOSIT,
		Quantity:        amount,
		Price:           10000, // USD deposit
		Symbol:          "USD",
		SymbolStockType: "USD",
		CreatedAt:       time.Now().Format(time.RFC3339),
		UpdatedAt:       time.Now().Format(time.RFC3339),
	}
	Transections = append(Transections, transection)

	return nil
}

func cancelOrder(cancelReq types.CancelOrderProps) error {
	// Find the order
	var orderToCancel *types.Order

	for i, order := range Orders {
		if order.Id == cancelReq.OrderId && order.UserId == cancelReq.UserId {
			orderToCancel = &Orders[i]
			break
		}
	}

	if orderToCancel == nil {
		return fmt.Errorf("order not found")
	}

	// Check if order can be cancelled (not completed)
	if orderToCancel.Status == types.COMPLETED {
		return fmt.Errorf("cannot cancel completed order")
	}

	// Remove from order book
	err := removeFromOrderBook(cancelReq.OrderId, cancelReq.StockSymbol, cancelReq.StockType, cancelReq.Price)
	if err != nil {
		return err
	}

	// Update order status
	orderToCancel.Status = types.CANCELLED
	orderToCancel.UpdatedAt = time.Now().Format(time.RFC3339)

	// Unlock balance if order was not filled
	if orderToCancel.FilledQty == 0 {
		// Find user balance and unlock the amount
		for i := range Balances {
			if Balances[i].UserId == cancelReq.UserId {
				lockedAmount := orderToCancel.Quantity * orderToCancel.Price
				Balances[i].Locked -= lockedAmount
				Balances[i].Balance += lockedAmount
				break
			}
		}
	}

	// Create cancellation transaction record
	transectionId, err := gonanoid.New()
	if err != nil {
		return err
	}

	transection := types.Transection{
		Id:              transectionId,
		MakerId:         cancelReq.UserId,
		TakerId:         cancelReq.UserId,
		TransectionType: types.CANCLE,
		Quantity:        orderToCancel.Quantity - orderToCancel.FilledQty, // Cancel unfilled quantity
		Price:           orderToCancel.Price,
		Symbol:          cancelReq.StockSymbol,
		SymbolStockType: cancelReq.StockType,
		CreatedAt:       time.Now().Format(time.RFC3339),
		UpdatedAt:       time.Now().Format(time.RFC3339),
	}
	Transections = append(Transections, transection)

	return nil
}

func endMarket(marketId string, stockSymbol string, winningStock string) error {
	// Find the market
	var marketToEnd *types.Market

	for i, market := range Markets {
		if market.Id == marketId {
			marketToEnd = &Markets[i]
			break
		}
	}

	if marketToEnd == nil {
		return fmt.Errorf("market not found")
	}

	// Update market status (we'll need to enhance the Market struct for this)
	// For now, we'll mark it as completed by updating the EndEventAfterTime
	marketToEnd.EndEventAfterTime = time.Now().Format(time.RFC3339)
	marketToEnd.UpdatedAt = time.Now().Format(time.RFC3339)

	// Process settlements based on winning stock
	// This is a simplified version - in a real system, you'd settle all outstanding orders
	winningStockLower := strings.ToLower(winningStock)

	// Find all orders for this symbol and settle them
	for i := range Orders {
		if Orders[i].Symbol == stockSymbol && Orders[i].Status == types.PENDING {
			// Determine if this order wins
			orderStockType := strings.ToLower(string(Orders[i].SymbolStockType))
			if orderStockType == winningStockLower {
				// Winning order - mark as completed and credit balance
				Orders[i].Status = types.COMPLETED
				Orders[i].FilledQty = Orders[i].Quantity
				Orders[i].UpdatedAt = time.Now().Format(time.RFC3339)

				// Credit the winner
				for j := range Balances {
					if Balances[j].UserId == Orders[i].UserId {
						// Winner gets their stake back plus profit
						// Simplified: just unlock the balance
						Balances[j].Locked -= Orders[i].Quantity * Orders[i].Price
						break
					}
				}
			} else {
				// Losing order - mark as completed, no payout
				Orders[i].Status = types.COMPLETED
				Orders[i].FilledQty = Orders[i].Quantity
				Orders[i].UpdatedAt = time.Now().Format(time.RFC3339)

				// Unlock balance (money is lost)
				for j := range Balances {
					if Balances[j].UserId == Orders[i].UserId {
						Balances[j].Locked -= Orders[i].Quantity * Orders[i].Price
						break
					}
				}
			}
		}
	}

	// Create market end transaction record
	transectionId, err := gonanoid.New()
	if err != nil {
		return err
	}

	transection := types.Transection{
		Id:              transectionId,
		MakerId:         "SYSTEM",
		TakerId:         "SYSTEM",
		TransectionType: types.CANCLE, // Using CANCLE as market settlement
		Quantity:        1,
		Price:           0,
		Symbol:          stockSymbol,
		SymbolStockType: winningStock,
		CreatedAt:       time.Now().Format(time.RFC3339),
		UpdatedAt:       time.Now().Format(time.RFC3339),
	}
	Transections = append(Transections, transection)

	return nil
}

func sendMessageToDatabaseQueueAndAwaitForResponseViaPubSubClient(message []byte, key string) error {
	err := engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", message).Err()
	if err != nil {
		return err
	}

	ch := make(chan string, 1)
	EngineAwaitsForResponseMap[key] = ch

	<-ch
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

			// Update order book if order was filled
			if data.FilledQty > 0 {
				updateOrderBookAfterFill(data.Id, data.FilledQty, data.Symbol, string(data.SymbolStockType), data.Price)
			}
			return nil
		}
	}
	data.CreatedAt = time.Now().Format(time.RFC3339)
	data.UpdatedAt = data.CreatedAt
	Orders = append(Orders, data)

	// Add new order to order book
	addToOrderBook(data)

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

// Order Book Management Functions
func addToOrderBook(order types.Order) error {
	symbol := order.Symbol
	stockType := strings.ToLower(string(order.SymbolStockType))

	// Initialize order book for symbol if it doesn't exist
	if _, exists := OrderBook[symbol]; !exists {
		OrderBook[symbol] = types.OrderBookPerStock{
			Yes: make(types.OrderBookPrices),
			No:  make(types.OrderBookPrices),
		}
	}

	// Get the appropriate price map (yes/no)
	var priceMap types.OrderBookPrices
	if stockType == "yes" {
		priceMap = OrderBook[symbol].Yes
	} else {
		priceMap = OrderBook[symbol].No
	}

	// Initialize price level if it doesn't exist
	if _, exists := priceMap[order.Price]; !exists {
		priceMap[order.Price] = types.OrderBookPerPrice{
			Total:  0,
			Orders: make(types.OrderBookOrders),
		}
	}

	// Add or update order
	priceLevel := priceMap[order.Price]
	priceLevel.Orders[order.Id] = types.OrderDetails{
		UserId:   order.UserId,
		Quantity: order.Quantity,
		Type:     "regular",
	}

	// Update total quantity for this price level
	total := 0.0
	for _, orderDetail := range priceLevel.Orders {
		total += orderDetail.Quantity
	}
	priceLevel.Total = total
	priceMap[order.Price] = priceLevel

	return nil
}

func removeFromOrderBook(orderId string, symbol string, stockType string, price float64) error {
	stockType = strings.ToLower(stockType)

	if _, exists := OrderBook[symbol]; !exists {
		return fmt.Errorf("symbol not found in order book")
	}

	var priceMap types.OrderBookPrices
	if stockType == "yes" {
		priceMap = OrderBook[symbol].Yes
	} else {
		priceMap = OrderBook[symbol].No
	}

	if _, exists := priceMap[price]; !exists {
		return fmt.Errorf("price level not found in order book")
	}

	// Remove the order
	priceLevel := priceMap[price]
	delete(priceLevel.Orders, orderId)

	// Update total quantity
	total := 0.0
	for _, orderDetail := range priceLevel.Orders {
		total += orderDetail.Quantity
	}
	priceLevel.Total = total
	priceMap[price] = priceLevel

	// Remove price level if no orders left
	if len(priceLevel.Orders) == 0 {
		delete(priceMap, price)
	}

	return nil
}

func getOrderBook(symbol string) (types.OrderBookPerStock, error) {
	if orderBook, exists := OrderBook[symbol]; exists {
		return orderBook, nil
	}
	return types.OrderBookPerStock{
		Yes: make(types.OrderBookPrices),
		No:  make(types.OrderBookPrices),
	}, nil
}

func updateOrderBookAfterFill(orderId string, filledQty float64, symbol string, stockType string, price float64) error {
	stockType = strings.ToLower(stockType)

	if _, exists := OrderBook[symbol]; !exists {
		return fmt.Errorf("order book entry not found")
	}

	var priceMap types.OrderBookPrices
	if stockType == "yes" {
		priceMap = OrderBook[symbol].Yes
	} else {
		priceMap = OrderBook[symbol].No
	}

	if _, exists := priceMap[price]; !exists {
		return fmt.Errorf("price level not found")
	}

	priceLevel := priceMap[price]
	if orderDetail, exists := priceLevel.Orders[orderId]; exists {
		// Reduce quantity
		orderDetail.Quantity -= filledQty
		priceLevel.Orders[orderId] = orderDetail

		// Update total
		priceLevel.Total -= filledQty
		priceMap[price] = priceLevel

		// Remove order if fully filled
		if orderDetail.Quantity <= 0 {
			delete(priceLevel.Orders, orderId)
			priceMap[price] = priceLevel
		}

		// Remove price level if no orders left
		if len(priceLevel.Orders) == 0 {
			delete(priceMap, price)
		}
	}

	return nil
}
