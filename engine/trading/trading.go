package trading

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/adityadeshlahre/probo-v1/engine/orderbook"
	types "github.com/adityadeshlahre/probo-v1/shared/types"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/redis/go-redis/v9"
)

var engineToDatabaseQueueClient *redis.Client
var engineToServerPubSubClient *redis.Client

// SetClients sets the Redis clients for trading operations
func SetClients(dbClient, pubSubClient *redis.Client) {
	engineToDatabaseQueueClient = dbClient
	engineToServerPubSubClient = pubSubClient
}

// Import these from main (will need to be passed or made accessible)
var USDBalances types.USDBalances
var StockBalances types.StockBalances
var OrderBook types.YesNoOrderBook

// SetDataStructures sets references to shared data structures
func SetDataStructures(usdBalances types.USDBalances, stockBalances types.StockBalances, orderBook types.YesNoOrderBook) {
	USDBalances = usdBalances
	StockBalances = stockBalances
	OrderBook = orderBook
}

// sendUSDBalancesToDB sends USD and stock balance updates to database
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

// mintStocks creates new stocks when no sellers exist
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

// swapStocks transfers existing stocks between users
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

// placeBuyOrder handles buy order placement and matching
func PlaceBuyOrder(orderData types.OrderProps) (map[string]interface{}, error) {
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
		"status":  true,
		"message": "Successfully placed the buy order",
		"stocks":  StockBalances[userId][stockSymbol],
	}, nil
}

// placeSellOrder handles sell order placement
func PlaceSellOrder(orderData types.OrderProps) (map[string]interface{}, error) {
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
	}, nil
}

// cancelOrder handles order cancellation
func CancelOrder(cancelReq types.CancelOrderProps) error {
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
	err := orderbook.RemoveFromOrderBook(orderId, stockSymbol, stockType, price)
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
