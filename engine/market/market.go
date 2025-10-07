package market

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	types "github.com/adityadeshlahre/probo-v1/shared/types"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/redis/go-redis/v9"
)

var engineToDatabaseQueueClient *redis.Client
var engineToServerPubSubClient *redis.Client

// SetClients sets the Redis clients for market operations
func SetClients(dbClient, pubSubClient *redis.Client) {
	engineToDatabaseQueueClient = dbClient
	engineToServerPubSubClient = pubSubClient
}

// Import these from main (will need to be passed or made accessible)
var USDBalances types.USDBalances
var StockBalances types.StockBalances
var OrderBook types.YesNoOrderBook
var MarketsMap types.Markets
var Orders []types.Order
var Transections []types.Transection

// SetDataStructures sets references to shared data structures
func SetDataStructures(usdBalances types.USDBalances, stockBalances types.StockBalances, orderBook types.YesNoOrderBook, marketsMap types.Markets, orders *[]types.Order, transections *[]types.Transection) {
	USDBalances = usdBalances
	StockBalances = stockBalances
	OrderBook = orderBook
	MarketsMap = marketsMap
	Orders = *orders
	Transections = *transections
}

// processWinnings handles payout to winners when market ends
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

// clearOrderBook removes all orders for a symbol and unlocks balances
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

// processOrder unlocks balances when clearing order book
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

// CreateMarket creates a new prediction market
func CreateMarket(createReq types.CreateMarket) error {
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

// EndMarket settles a market and processes winnings
func EndMarket(stockSymbol string, winningStock string) error {
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
