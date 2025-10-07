package balance

import (
	"time"

	types "github.com/adityadeshlahre/probo-v1/shared/types"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/redis/go-redis/v9"
)

var engineToDatabaseQueueClient *redis.Client

// SetClients sets the Redis clients for balance operations
func SetClients(dbClient *redis.Client) {
	engineToDatabaseQueueClient = dbClient
}

// Import these from main (will need to be passed or made accessible)
var USDBalances types.USDBalances
var StockBalances types.StockBalances
var Balances []types.Balance
var Transections []types.Transection

// SetDataStructures sets references to shared data structures
func SetDataStructures(usdBalances types.USDBalances, stockBalances types.StockBalances, balances *[]types.Balance, transections *[]types.Transection) {
	USDBalances = usdBalances
	StockBalances = stockBalances
	Balances = *balances
	Transections = *transections
}

// OnRampUSD adds USD to user balance
func OnRampUSD(userId string, amount float64) error {
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
