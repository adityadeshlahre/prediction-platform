package orderbook

import (
	"fmt"
	"strings"

	types "github.com/adityadeshlahre/probo-v1/shared/types"
)

// Import these from main (will need to be passed or made accessible)
var OrderBook types.YesNoOrderBook

// SetDataStructures sets references to shared data structures
func SetDataStructures(orderBook types.YesNoOrderBook) {
	OrderBook = orderBook
}

// addToOrderBook adds an order to the order book
func AddToOrderBook(order types.Order) error {
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
		Price:    order.Price,
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

// RemoveFromOrderBook removes an order from the order book
func RemoveFromOrderBook(orderId string, symbol string, stockType string, price float64) error {
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

// GetOrderBook returns the order book for a symbol
func GetOrderBook(symbol string) (types.SymbolOrderBook, error) {
	if orderBook, exists := OrderBook[symbol]; exists {
		return orderBook, nil
	}
	return types.SymbolOrderBook{
		Yes: make(types.PriceOrderBook),
		No:  make(types.PriceOrderBook),
	}, nil
}

// GetAllOrderBooks returns all order books
func GetAllOrderBooks() (types.YesNoOrderBook, error) {
	return OrderBook, nil
}

// UpdateOrderBookAfterFill updates the order book after an order is filled
func UpdateOrderBookAfterFill(orderId string, filledQty float64, symbol string, stockType string, price float64) error {
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
