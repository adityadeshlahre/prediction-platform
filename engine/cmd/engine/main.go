package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
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

var USDBalances types.USDBalances = make(types.USDBalances)
var StockBalances types.StockBalances = make(types.StockBalances)
var OrderBook types.YesNoOrderBook = make(types.YesNoOrderBook)
var MarketsMap types.Markets = make(types.Markets)

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
		err = endMarket(endReq.StockSymbol, endReq.WinningStock)
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
		result, err := placeBuyOrder(orderProps)
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
		result, err := placeSellOrder(orderProps)
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
		err = createMarket(createReq)
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

func sendUSDBalancesToDB() {
	// Send USD balances update
	usdData := make(map[string]interface{})
	for userId, balance := range USDBalances {
		usdData[userId] = balance
	}

	usdMsg := types.IncomingMessage{
		Type: string(types.USER_USD),
		Data: json.RawMessage(fmt.Sprintf(`{"data":%s}`, mustMarshal(usdData))),
	}
	usdBytes, _ := json.Marshal(usdMsg)
	engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", usdBytes)

	// Send stock balances update
	stockData := make(map[string]interface{})
	for userId, userStocks := range StockBalances {
		stockData[userId] = map[string]interface{}{
			"data": userStocks,
		}
	}

	stockMsg := types.IncomingMessage{
		Type: string(types.USER_STOCKS),
		Data: json.RawMessage(fmt.Sprintf(`{"data":%s}`, mustMarshal(stockData))),
	}
	stockBytes, _ := json.Marshal(stockMsg)
	engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", stockBytes)
}

func mustMarshal(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func mintStocks(userId, stockSymbol, sellerId string, price float64, stockType string, availableQuantity float64) error {
	oppositeStockType := "no"
	if stockType == "yes" {
		oppositeStockType = "no"
	} else {
		oppositeStockType = "yes"
	}
	correspondingPrice := 10.0 - price

	// Initialize stock balances if they don't exist
	if _, exists := StockBalances[sellerId]; !exists {
		StockBalances[sellerId] = make(types.UserStockBalance)
	}
	if _, exists := StockBalances[sellerId][stockSymbol]; !exists {
		StockBalances[sellerId][stockSymbol] = types.SymbolStockBalance{
			Yes: types.StockPosition{Quantity: 0, Locked: 0},
			No:  types.StockPosition{Quantity: 0, Locked: 0},
		}
	}
	if _, exists := StockBalances[userId]; !exists {
		StockBalances[userId] = make(types.UserStockBalance)
	}
	if _, exists := StockBalances[userId][stockSymbol]; !exists {
		StockBalances[userId][stockSymbol] = types.SymbolStockBalance{
			Yes: types.StockPosition{Quantity: 0, Locked: 0},
			No:  types.StockPosition{Quantity: 0, Locked: 0},
		}
	}

	// Mint stocks: seller gets opposite type, buyer gets requested type
	if oppositeStockType == "yes" {
		sellerStock := StockBalances[sellerId][stockSymbol]
		sellerStock.Yes.Quantity += availableQuantity
		StockBalances[sellerId][stockSymbol] = sellerStock
	} else {
		sellerStock := StockBalances[sellerId][stockSymbol]
		sellerStock.No.Quantity += availableQuantity
		StockBalances[sellerId][stockSymbol] = sellerStock
	}

	if stockType == "yes" {
		buyerStock := StockBalances[userId][stockSymbol]
		buyerStock.Yes.Quantity += availableQuantity
		StockBalances[userId][stockSymbol] = buyerStock
	} else {
		buyerStock := StockBalances[userId][stockSymbol]
		buyerStock.No.Quantity += availableQuantity
		StockBalances[userId][stockSymbol] = buyerStock
	}

	// Update USD balances
	if buyerBalance, exists := USDBalances[userId]; exists {
		buyerBalance.Balance -= availableQuantity * price
		USDBalances[userId] = buyerBalance
	}

	if sellerBalance, exists := USDBalances[sellerId]; exists {
		sellerBalance.Locked -= availableQuantity * correspondingPrice
		USDBalances[sellerId] = sellerBalance
	}

	// Send database updates
	sendUSDBalancesToDB()

	return nil
}

func swapStocks(userId, stockSymbol, sellerId string, price float64, stockType string, availableQuantity float64) error {
	// Initialize stock balances if they don't exist
	if _, exists := StockBalances[userId]; !exists {
		StockBalances[userId] = make(types.UserStockBalance)
	}
	if _, exists := StockBalances[userId][stockSymbol]; !exists {
		StockBalances[userId][stockSymbol] = types.SymbolStockBalance{
			Yes: types.StockPosition{Quantity: 0, Locked: 0},
			No:  types.StockPosition{Quantity: 0, Locked: 0},
		}
	}

	// Unlock seller's stocks and transfer to buyer
	if stockType == "yes" {
		// Update seller's stocks
		if sellerStocks, exists := StockBalances[sellerId]; exists {
			if symbolStocks, exists := sellerStocks[stockSymbol]; exists {
				symbolStocks.Yes.Locked -= availableQuantity
				sellerStocks[stockSymbol] = symbolStocks
				StockBalances[sellerId] = sellerStocks
			}
		}

		// Update buyer's stocks
		buyerStocks := StockBalances[userId]
		symbolStocks := buyerStocks[stockSymbol]
		symbolStocks.Yes.Quantity += availableQuantity
		buyerStocks[stockSymbol] = symbolStocks
		StockBalances[userId] = buyerStocks
	} else {
		// Update seller's stocks
		if sellerStocks, exists := StockBalances[sellerId]; exists {
			if symbolStocks, exists := sellerStocks[stockSymbol]; exists {
				symbolStocks.No.Locked -= availableQuantity
				sellerStocks[stockSymbol] = symbolStocks
				StockBalances[sellerId] = sellerStocks
			}
		}

		// Update buyer's stocks
		buyerStocks := StockBalances[userId]
		symbolStocks := buyerStocks[stockSymbol]
		symbolStocks.No.Quantity += availableQuantity
		buyerStocks[stockSymbol] = symbolStocks
		StockBalances[userId] = buyerStocks
	}

	// Update USD balances
	if buyerBalance, exists := USDBalances[userId]; exists {
		buyerBalance.Balance -= availableQuantity * price
		USDBalances[userId] = buyerBalance
	}

	if sellerBalance, exists := USDBalances[sellerId]; exists {
		sellerBalance.Balance += availableQuantity * price
		USDBalances[sellerId] = sellerBalance
	}

	// Send database updates
	sendUSDBalancesToDB()

	return nil
}

func processWinnings(stockSymbol string, winningStock string) error {
	fmt.Printf("Processing winnings for %s, winner: %s\n", stockSymbol, winningStock)

	// Process payouts for all users
	for userId, userStocks := range StockBalances {
		if symbolStocks, exists := userStocks[stockSymbol]; exists {
			var winningQuantity float64
			if winningStock == "yes" {
				winningQuantity = symbolStocks.Yes.Quantity
			} else {
				winningQuantity = symbolStocks.No.Quantity
			}

			// Pay out winners: quantity * 1000
			if winningQuantity > 0 {
				if balance, exists := USDBalances[userId]; exists {
					balance.Balance += winningQuantity * 1000
					USDBalances[userId] = balance
				}
			}

			// Clear stock balances for this symbol
			delete(userStocks, stockSymbol)
			StockBalances[userId] = userStocks
		}
	}

	// Send database updates
	sendUSDBalancesToDB()

	return nil
}

func clearOrderBook(stockSymbol string) error {
	fmt.Printf("Clearing order book for %s\n", stockSymbol)

	if _, exists := OrderBook[stockSymbol]; !exists {
		return fmt.Errorf("order book for symbol '%s' doesn't exist", stockSymbol)
	}

	orderBook := OrderBook[stockSymbol]

	// Process all orders in the YES order book
	for price, entry := range orderBook.Yes {
		for _, order := range entry.Orders {
			processOrder(order, stockSymbol, "yes", price)
		}
	}

	// Process all orders in the NO order book
	for price, entry := range orderBook.No {
		for _, order := range entry.Orders {
			processOrder(order, stockSymbol, "no", price)
		}
	}

	// Clear the order book
	delete(OrderBook, stockSymbol)

	return nil
}

func processOrder(order types.OrderBookEntry, stockSymbol string, stockType string, price float64) error {
	if _, exists := USDBalances[order.UserId]; !exists {
		return fmt.Errorf("invalid balances for user %s", order.UserId)
	}

	if _, exists := StockBalances[order.UserId]; !exists {
		return fmt.Errorf("invalid stock balances for user %s", order.UserId)
	}

	fmt.Printf("Processing order for user %s\n", order.UserId)

	switch order.Type {
	case "reverted":
		// Unlock USD balance
		if balance, exists := USDBalances[order.UserId]; exists {
			balance.Locked -= order.Quantity * price
			balance.Balance += order.Quantity * price
			USDBalances[order.UserId] = balance
		}
	case "regular":
		// Unlock stock balance
		if userStocks, exists := StockBalances[order.UserId]; exists {
			if symbolStocks, exists := userStocks[stockSymbol]; exists {
				if stockType == "yes" {
					symbolStocks.Yes.Locked -= order.Quantity
					symbolStocks.Yes.Quantity += order.Quantity
				} else {
					symbolStocks.No.Locked -= order.Quantity
					symbolStocks.No.Quantity += order.Quantity
				}
				userStocks[stockSymbol] = symbolStocks
				StockBalances[order.UserId] = userStocks
			}
		}
	default:
		return fmt.Errorf("unknown order type: %s", order.Type)
	}

	return nil
}

func placeBuyOrder(orderData types.OrderProps) (map[string]interface{}, error) {
	userId := orderData.UserId
	stockSymbol := orderData.StockSymbol
	quantity := orderData.Quantity
	price := orderData.Price
	stockType := orderData.StockType

	// Price conversion: USD to stock price (0-10 range)
	if price > 1000 || price < 0 {
		return nil, fmt.Errorf("invalid price, price should be between 0 and 10")
	}
	stockPrice := price / 100
	oppositeStockType := "no"
	if stockType == "yes" {
		oppositeStockType = "no"
	} else {
		oppositeStockType = "yes"
	}

	// Validate user balance
	if _, exists := USDBalances[userId]; !exists {
		return nil, fmt.Errorf("user with the given id doesn't exist")
	}

	// Initialize order book for symbol if it doesn't exist
	if _, exists := OrderBook[stockSymbol]; !exists {
		OrderBook[stockSymbol] = types.SymbolOrderBook{
			Yes: make(types.PriceOrderBook),
			No:  make(types.PriceOrderBook),
		}
	}

	// Check sufficient balance
	if USDBalances[userId].Balance < quantity*price {
		return nil, fmt.Errorf("insufficient balance")
	}

	// Initialize stock balances
	if _, exists := StockBalances[userId]; !exists {
		StockBalances[userId] = make(types.UserStockBalance)
	}
	if _, exists := StockBalances[userId][stockSymbol]; !exists {
		StockBalances[userId][stockSymbol] = types.SymbolStockBalance{
			Yes: types.StockPosition{Quantity: 0, Locked: 0},
			No:  types.StockPosition{Quantity: 0, Locked: 0},
		}
	}

	requiredQuantity := quantity
	orderId, _ := gonanoid.New()

	// Create order record
	orderRecord := types.Order{
		Id:              orderId,
		UserId:          userId,
		OrderType:       types.BUY,
		Symbol:          stockSymbol,
		SymbolStockType: stockType,
		Price:           stockPrice,
		Quantity:        requiredQuantity,
		FilledQty:       0,
		Status:          types.PENDING,
		CreatedAt:       time.Now().Format(time.RFC3339),
		UpdatedAt:       time.Now().Format(time.RFC3339),
	}

	// Send to database
	orderDataBytes, _ := json.Marshal(orderRecord)
	orderMsg := types.IncomingMessage{
		Type: "ORDER",
		Data: orderDataBytes,
	}
	orderMsgBytes, _ := json.Marshal(orderMsg)
	engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", orderMsgBytes)

	// Try to match with existing sell orders
	var priceMap types.PriceOrderBook
	if stockType == "yes" {
		priceMap = OrderBook[stockSymbol].Yes
	} else {
		priceMap = OrderBook[stockSymbol].No
	}

	if entry, exists := priceMap[stockPrice]; exists && entry.Total >= requiredQuantity {
		// Match with existing sell orders
		entry.Total -= requiredQuantity

		for sellOrderId, sellerOrder := range entry.Orders {
			if sellerOrder.Quantity > 0 {
				availableQuantity := math.Min(sellerOrder.Quantity, requiredQuantity)

				if sellerOrder.Type == "reverted" {
					// Mint new stocks
					mintStocks(userId, stockSymbol, sellerOrder.UserId, stockPrice, stockType, availableQuantity)
				} else {
					// Swap existing stocks
					swapStocks(userId, stockSymbol, sellerOrder.UserId, stockPrice, stockType, availableQuantity)
				}

				// Update order records
				updateOrderData := map[string]interface{}{
					"orderId":   sellOrderId,
					"filledQty": availableQuantity,
					"status":    sellerOrder.Quantity == availableQuantity,
				}
				updateBytes, _ := json.Marshal(updateOrderData)
				updateMsg := types.IncomingMessage{
					Type: "UPDATE_ORDER",
					Data: updateBytes,
				}
				updateMsgBytes, _ := json.Marshal(updateMsg)
				engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", updateMsgBytes)

				requiredQuantity -= availableQuantity

				// Update buy order
				buyUpdateData := map[string]interface{}{
					"orderId":   orderId,
					"filledQty": availableQuantity,
					"status":    requiredQuantity == 0,
				}
				buyUpdateBytes, _ := json.Marshal(buyUpdateData)
				buyUpdateMsg := types.IncomingMessage{
					Type: "UPDATE_ORDER",
					Data: buyUpdateBytes,
				}
				buyUpdateMsgBytes, _ := json.Marshal(buyUpdateMsg)
				engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", buyUpdateMsgBytes)

				sellerOrder.Quantity -= availableQuantity
				entry.Orders[sellOrderId] = sellerOrder

				if requiredQuantity == 0 {
					break
				}
			}
		}

		// Update order book
		priceMap[stockPrice] = entry

		// Send WebSocket updates
		orderBookData, _ := json.Marshal(OrderBook[stockSymbol])
		wsMsg := types.IncomingMessage{
			Type: "ORDER_BOOK_UPDATE",
			Data: orderBookData,
		}
		wsBytes, _ := json.Marshal(wsMsg)
		engineToServerPubSubClient.Publish(context.Background(), stockSymbol, wsBytes)

		return map[string]interface{}{
			"status":    true,
			"message":   "Successfully bought the required quantity",
			"stocks":    StockBalances[userId][stockSymbol],
			"orderbook": OrderBook[stockSymbol],
		}, nil
	}

	// No matching sell orders - create reverted order
	correspondingPrice := 10.0 - stockPrice
	symbolOrderBook := OrderBook[stockSymbol]

	var oppositePriceMap types.PriceOrderBook
	if oppositeStockType == "yes" {
		if symbolOrderBook.Yes == nil {
			symbolOrderBook.Yes = make(types.PriceOrderBook)
		}
		oppositePriceMap = symbolOrderBook.Yes
	} else {
		if symbolOrderBook.No == nil {
			symbolOrderBook.No = make(types.PriceOrderBook)
		}
		oppositePriceMap = symbolOrderBook.No
	}

	if _, exists := oppositePriceMap[correspondingPrice]; !exists {
		oppositePriceMap[correspondingPrice] = types.PriceLevel{
			Total:  0,
			Orders: make(map[string]types.OrderBookEntry),
		}
	}

	oppositeEntry := oppositePriceMap[correspondingPrice]
	oppositeEntry.Total += requiredQuantity
	oppositeEntry.Orders[orderId] = types.OrderBookEntry{
		UserId:   userId,
		Quantity: requiredQuantity,
		Type:     "reverted",
	}
	oppositePriceMap[correspondingPrice] = oppositeEntry

	// Update the symbol order book
	if oppositeStockType == "yes" {
		symbolOrderBook.Yes = oppositePriceMap
	} else {
		symbolOrderBook.No = oppositePriceMap
	}
	OrderBook[stockSymbol] = symbolOrderBook

	// Lock buyer balance
	userBalance := USDBalances[userId]
	userBalance.Locked += requiredQuantity * price
	userBalance.Balance -= requiredQuantity * price
	USDBalances[userId] = userBalance

	// Send balance update
	sendUSDBalancesToDB()

	// Send WebSocket updates
	orderBookData, _ := json.Marshal(OrderBook[stockSymbol])
	wsMsg := types.IncomingMessage{
		Type: "ORDER_BOOK_UPDATE",
		Data: orderBookData,
	}
	wsBytes, _ := json.Marshal(wsMsg)
	engineToServerPubSubClient.Publish(context.Background(), stockSymbol, wsBytes)

	return map[string]interface{}{
		"status":    true,
		"message":   "Successfully placed the buy order",
		"stocks":    StockBalances[userId][stockSymbol],
		"orderbook": OrderBook[stockSymbol],
	}, nil
}

func placeSellOrder(orderData types.OrderProps) (map[string]interface{}, error) {
	userId := orderData.UserId
	stockSymbol := orderData.StockSymbol
	quantity := orderData.Quantity
	price := orderData.Price
	stockType := orderData.StockType

	if price > 1000 || price < 0 {
		return nil, fmt.Errorf("price should be between 0 and 10 USD")
	}
	stockPrice := price / 100

	if _, exists := USDBalances[userId]; !exists {
		return nil, fmt.Errorf("user with the given id doesn't exist")
	}

	// Initialize stock balances if needed
	if _, exists := StockBalances[userId]; !exists {
		return nil, fmt.Errorf("user doesn't have the required stocks to sell")
	}
	if _, exists := StockBalances[userId][stockSymbol]; !exists {
		return nil, fmt.Errorf("user doesn't have stocks for this symbol")
	}

	// Check if user has sufficient stocks
	userStocks := StockBalances[userId][stockSymbol]
	var availableQuantity float64
	if stockType == "yes" {
		availableQuantity = userStocks.Yes.Quantity
	} else {
		availableQuantity = userStocks.No.Quantity
	}

	if availableQuantity < quantity {
		return nil, fmt.Errorf("user doesn't have the required quantity")
	}

	// Initialize order book
	if _, exists := OrderBook[stockSymbol]; !exists {
		OrderBook[stockSymbol] = types.SymbolOrderBook{
			Yes: make(types.PriceOrderBook),
			No:  make(types.PriceOrderBook),
		}
	}

	symbolOrderBook := OrderBook[stockSymbol]
	var priceMap types.PriceOrderBook
	if stockType == "yes" {
		priceMap = symbolOrderBook.Yes
	} else {
		priceMap = symbolOrderBook.No
	}

	if _, exists := priceMap[stockPrice]; !exists {
		priceMap[stockPrice] = types.PriceLevel{
			Total:  0,
			Orders: make(map[string]types.OrderBookEntry),
		}
	}

	// Generate order ID
	orderId, _ := gonanoid.New()

	// Add to order book
	priceLevel := priceMap[stockPrice]
	priceLevel.Total += quantity
	priceLevel.Orders[orderId] = types.OrderBookEntry{
		Quantity: quantity,
		Type:     "regular",
		UserId:   userId,
	}
	priceMap[stockPrice] = priceLevel

	// Update symbol order book
	if stockType == "yes" {
		symbolOrderBook.Yes = priceMap
	} else {
		symbolOrderBook.No = priceMap
	}
	OrderBook[stockSymbol] = symbolOrderBook

	// Lock user stocks
	if stockType == "yes" {
		userStocks.Yes.Quantity -= quantity
		userStocks.Yes.Locked += quantity
	} else {
		userStocks.No.Quantity -= quantity
		userStocks.No.Locked += quantity
	}
	StockBalances[userId][stockSymbol] = userStocks

	// Create order record
	orderRecord := types.Order{
		Id:              orderId,
		UserId:          userId,
		OrderType:       types.SELL,
		Symbol:          stockSymbol,
		SymbolStockType: stockType,
		Price:           stockPrice,
		Quantity:        quantity,
		FilledQty:       0,
		Status:          types.PENDING,
		CreatedAt:       time.Now().Format(time.RFC3339),
		UpdatedAt:       time.Now().Format(time.RFC3339),
	}

	// Send to database
	orderDataBytes, _ := json.Marshal(orderRecord)
	orderMsg := types.IncomingMessage{
		Type: "ORDER",
		Data: orderDataBytes,
	}
	orderMsgBytes, _ := json.Marshal(orderMsg)
	engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", orderMsgBytes)

	// Send stock balance update
	sendUSDBalancesToDB()

	return map[string]interface{}{
		"status":      true,
		"message":     "Successfully placed the sell order",
		"stockSymbol": stockSymbol,
		"orderbook":   OrderBook[stockSymbol],
	}, nil
}

func cancelOrder(cancelReq types.CancelOrderProps) error {
	userId := cancelReq.UserId
	stockSymbol := cancelReq.StockSymbol
	price := cancelReq.Price
	orderId := cancelReq.OrderId
	stockType := cancelReq.StockType

	if _, exists := USDBalances[userId]; !exists {
		return fmt.Errorf("user with the given id doesn't exist")
	}

	if _, exists := OrderBook[stockSymbol]; !exists {
		return fmt.Errorf("order book doesn't exist")
	}

	var priceMap types.PriceOrderBook
	if stockType == "yes" {
		priceMap = OrderBook[stockSymbol].Yes
	} else {
		priceMap = OrderBook[stockSymbol].No
	}

	if _, exists := priceMap[price]; !exists {
		return fmt.Errorf("price level not found")
	}

	priceLevel := priceMap[price]
	order, exists := priceLevel.Orders[orderId]
	if !exists {
		return fmt.Errorf("order doesn't exist")
	}

	if order.UserId != userId {
		return fmt.Errorf("user doesn't have permission to cancel this order")
	}

	// Unlock balances based on order type
	if order.Type == "reverted" {
		// Unlock USD balance (from buy order)
		userBalance := USDBalances[userId]
		userBalance.Locked -= order.Quantity * price
		userBalance.Balance += order.Quantity * price
		USDBalances[userId] = userBalance
	} else {
		// Unlock stocks (from sell order)
		if userStocks, exists := StockBalances[userId]; exists {
			if symbolStocks, exists := userStocks[stockSymbol]; exists {
				if stockType == "yes" {
					symbolStocks.Yes.Locked -= order.Quantity
					symbolStocks.Yes.Quantity += order.Quantity
				} else {
					symbolStocks.No.Locked -= order.Quantity
					symbolStocks.No.Quantity += order.Quantity
				}
				userStocks[stockSymbol] = symbolStocks
				StockBalances[userId] = userStocks
			}
		}
	}

	// Remove from order book
	err := removeFromOrderBook(orderId, stockSymbol, stockType, price)
	if err != nil {
		return err
	}

	// Send database updates
	sendUSDBalancesToDB()

	// Update order status in database
	orderUpdate := map[string]interface{}{
		"orderId": orderId,
		"status":  "CANCELLED",
	}
	orderBytes, _ := json.Marshal(orderUpdate)
	orderMsg := types.IncomingMessage{
		Type: "UPDATE_ORDER",
		Data: orderBytes,
	}
	orderMsgBytes, _ := json.Marshal(orderMsg)
	engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", orderMsgBytes)

	return nil
}

func endMarket(stockSymbol string, winningStock string) error {
	// Update market status in MarketsMap
	if market, exists := MarketsMap[stockSymbol]; exists {
		market.Status = types.MarketCompleted
		MarketsMap[stockSymbol] = market
	}

	// Process winnings for all users
	err := processWinnings(stockSymbol, strings.ToLower(winningStock))
	if err != nil {
		return fmt.Errorf("failed to process winnings: %v", err)
	}

	// Clear the order book and unlock all balances
	err = clearOrderBook(stockSymbol)
	if err != nil {
		return fmt.Errorf("failed to clear order book: %v", err)
	}

	// Update all pending orders for this symbol to cancelled
	for i := range Orders {
		if Orders[i].Symbol == stockSymbol && Orders[i].Status == types.PENDING {
			Orders[i].Status = types.CANCELLED
			Orders[i].UpdatedAt = time.Now().Format(time.RFC3339)

			// Send order update to database
			orderUpdate := map[string]interface{}{
				"orderId": Orders[i].Id,
				"status":  "CANCELLED",
			}
			orderBytes, _ := json.Marshal(orderUpdate)
			orderMsg := types.IncomingMessage{
				Type: "UPDATE_ORDER",
				Data: orderBytes,
			}
			orderMsgBytes, _ := json.Marshal(orderMsg)
			engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", orderMsgBytes)
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

	// Send transaction to database
	transectionData, _ := json.Marshal(transection)
	transectionMsg := types.IncomingMessage{
		Type: "TRANSECTION",
		Data: transectionData,
	}
	transectionMsgBytes, _ := json.Marshal(transectionMsg)
	engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", transectionMsgBytes)

	return nil
}

func createMarket(createReq types.CreateMarket) error {
	// Generate market ID
	marketId, err := gonanoid.New()
	if err != nil {
		return err
	}

	// Create market record
	market := types.Market{
		Id:                marketId,
		Symbol:            createReq.Symbol,
		SymbolStockType:   "YES_NO", // Default for prediction markets
		SourceOfTruth:     createReq.SourceOfTruth,
		Heading:           createReq.Heading,
		EventType:         createReq.EventType,
		RepeatEventTime:   fmt.Sprintf("%d", createReq.RepeatEventTime),
		EndEventAfterTime: fmt.Sprintf("%d", createReq.EndAfterTime),
		CreatedAt:         time.Now().Format(time.RFC3339),
		UpdatedAt:         time.Now().Format(time.RFC3339),
	}

	// Initialize order book for the market
	if _, exists := OrderBook[createReq.Symbol]; !exists {
		OrderBook[createReq.Symbol] = types.SymbolOrderBook{
			Yes: make(types.PriceOrderBook),
			No:  make(types.PriceOrderBook),
		}
	}

	// Initialize market in MarketsMap
	MarketsMap[createReq.Symbol] = types.EnhancedMarket{
		StockSymbol: createReq.Symbol,
		Price:       5.0, // Default starting price
		Heading:     createReq.Heading,
		EventType:   createReq.EventType,
		Type:        types.MarketType(createReq.MarketType),
		Status:      types.MarketActive,
	}

	// Send market to database
	marketData, err := json.Marshal(market)
	if err != nil {
		return err
	}
	marketMsg := types.IncomingMessage{
		Type: "MARKET",
		Data: marketData,
	}
	marketMsgBytes, _ := json.Marshal(marketMsg)
	engineToDatabaseQueueClient.Publish(context.Background(), "DB_ACTIONS", marketMsgBytes)

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
		OrderBook[symbol] = types.SymbolOrderBook{
			Yes: make(types.PriceOrderBook),
			No:  make(types.PriceOrderBook),
		}
	}

	// Get the appropriate price map (yes/no)
	var priceMap types.PriceOrderBook
	if stockType == "yes" {
		priceMap = OrderBook[symbol].Yes
	} else {
		priceMap = OrderBook[symbol].No
	}

	// Initialize price level if it doesn't exist
	if _, exists := priceMap[order.Price]; !exists {
		priceMap[order.Price] = types.PriceLevel{
			Total:  0,
			Orders: make(map[string]types.OrderBookEntry),
		}
	}

	// Add or update order
	priceLevel := priceMap[order.Price]
	priceLevel.Orders[order.Id] = types.OrderBookEntry{
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

	var priceMap types.PriceOrderBook
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

func getOrderBook(symbol string) (types.SymbolOrderBook, error) {
	if orderBook, exists := OrderBook[symbol]; exists {
		return orderBook, nil
	}
	return types.SymbolOrderBook{
		Yes: make(types.PriceOrderBook),
		No:  make(types.PriceOrderBook),
	}, nil
}

func updateOrderBookAfterFill(orderId string, filledQty float64, symbol string, stockType string, price float64) error {
	stockType = strings.ToLower(stockType)

	if _, exists := OrderBook[symbol]; !exists {
		return fmt.Errorf("order book entry not found")
	}

	var priceMap types.PriceOrderBook
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
