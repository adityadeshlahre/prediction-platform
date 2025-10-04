package wshandler

import (
	"fmt"
	"time"

	"github.com/adityadeshlahre/probo-v1/shared/types"
)

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
