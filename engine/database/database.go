package database

import (
	"fmt"
	"time"

	types "github.com/adityadeshlahre/probo-v1/shared/types"
)

// Import these from main (will need to be passed or made accessible)
var Orders []types.Order
var Users []types.User
var Balances []types.Balance
var Transections []types.Transection
var Markets []types.Market
var TransectionCounter int

// SetDataStructures sets references to shared data structures
func SetDataStructures(orders *[]types.Order, users *[]types.User, balances *[]types.Balance, transections *[]types.Transection, markets *[]types.Market, transectionCounter *int) {
	Orders = *orders
	Users = *users
	Balances = *balances
	Transections = *transections
	Markets = *markets
	TransectionCounter = *transectionCounter
}

// CreateOrUpdateOrder creates or updates an order
func CreateOrUpdateOrder(data types.Order) error {
	for i := range Orders {
		if Orders[i].Id == data.Id {
			Orders[i].FilledQty += data.FilledQty
			if data.Status != "" {
				Orders[i].Status = data.Status
			}
			Orders[i].UpdatedAt = time.Now().Format(time.RFC3339)

			// Update order book if order was filled
			if data.FilledQty > 0 {
				// This would need to be imported from orderbook package
				// updateOrderBookAfterFill(data.Id, data.FilledQty, data.Symbol, string(data.SymbolStockType), data.Price)
			}
			return nil
		}
	}
	data.CreatedAt = time.Now().Format(time.RFC3339)
	data.UpdatedAt = data.CreatedAt
	Orders = append(Orders, data)

	// Add new order to order book
	// This would need to be imported from orderbook package
	// addToOrderBook(data)

	return nil
}

// CreateOrUpdateBalance creates or updates a balance
func CreateOrUpdateBalance(data types.Balance) error {
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

// CreateOrUpdateUser creates or updates a user
func CreateOrUpdateUser(data types.User) error {
	for i := range Users {
		if Users[i].Id == data.Id {
			Users[i] = data
			return nil
		}
	}
	Users = append(Users, data)
	return nil
}

// CreateOrUpdateUserStock creates or updates user stock data
func CreateOrUpdateUserStock(data types.User) error {
	for i := range Users {
		if Users[i].Id == data.Id {
			Users[i].Stock = data.Stock
			return nil
		}
	}
	Users = append(Users, data)
	return nil
}

// CreateOrUpdateMarket creates or updates a market
func CreateOrUpdateMarket(data types.Market) error {
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

// CreateTransection creates a new transaction
func CreateTransection(data types.Transection) error {
	data.Id = fmt.Sprintf("transection%d", TransectionCounter)
	TransectionCounter++
	data.CreatedAt = time.Now().Format(time.RFC3339)
	data.UpdatedAt = data.CreatedAt
	Transections = append(Transections, data)
	return nil
}
