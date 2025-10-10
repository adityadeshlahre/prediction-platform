package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"time"

	types "github.com/adityadeshlahre/probo-v1/shared/types"
	"github.com/gorilla/websocket"
)

type BalanceResp struct {
	Type string `json:"type"`
	Data struct {
		Balance types.Balance `json:"balance"`
		UserId  string        `json:"userId"`
	} `json:"data"`
}

type StocksResp struct {
	Type string `json:"type"`
	Data struct {
		Stocks types.UserStockBalance `json:"stocks"`
		UserId string                 `json:"userId"`
	} `json:"data"`
}

const (
	serverURL = "http://localhost:8080"
	numUsers  = 10
)

func main() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	fmt.Println("Starting random integration tests for probo-v1 application...")

	// Wait for services to be ready
	time.Sleep(3 * time.Second)

	// Test 1: Create multiple users
	fmt.Println("Test 1: Creating multiple users...")
	userIDs := make([]string, numUsers)
	for i := 0; i < numUsers; i++ {
		userID := fmt.Sprintf("%d", i+1)
		userIDs[i] = userID
		resp, err := http.Post(serverURL+"/user/"+userID, "application/json", nil)
		if err != nil {
			log.Fatalf("Failed to create user %s: %v", userID, err)
		}
		if resp.StatusCode != 200 {
			log.Fatalf("Create user %s failed with status: %d", userID, resp.StatusCode)
		}
		resp.Body.Close()
		fmt.Printf("✓ User %s created\n", userID)
	}

	// Test 2: Create a manual market
	fmt.Println("Test 2: Creating a manual prediction market...")
	marketReq := types.CreateMarket{
		Symbol:        "BTC_PREDICT",
		MarketType:    "manual",
		EndsIn:        30000, // 30 seconds
		SourceOfTruth: "manual",
		EndAfterTime:  30000,
		Heading:       "Will BTC be above $100k?",
		EventType:     "crypto",
	}
	marketJSON, _ := json.Marshal(marketReq)
	resp, err := http.Post(serverURL+"/symbol/createmarket", "application/json", bytes.NewBuffer(marketJSON))
	if err != nil {
		log.Fatalf("Failed to create manual market: %v", err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		log.Fatalf("Create manual market failed with status: %d, body: %s", resp.StatusCode, string(body))
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var marketResp map[string]interface{}
	json.Unmarshal(body, &marketResp)
	marketSymbol := ""
	for key := range marketResp {
		if key != "status" {
			marketSymbol = key
			break
		}
	}
	if marketSymbol == "" {
		log.Fatalf("Failed to get market symbol from response")
	}
	fmt.Printf("✓ Manual market created successfully with symbol: %s\n", marketSymbol)

	// Test 2b: Create an automatic market (1 minute interval, using CoinGecko API)
	// Commented out for stress testing to avoid interference
	fmt.Println("Test 2b: Creating an automatic prediction market...")
	autoMarketReq := types.CreateMarket{
		Symbol:          "bitcoin", // Must be in allowed list
		MarketType:      "automatic",
		EndsIn:          120000, // 2 minutes for market duration
		SourceOfTruth:   "automatic",
		EndAfterTime:    120000,
		Heading:         "Will BTC price change significantly?",
		EventType:       "crypto",
		RepeatEventTime: 60000, // 1 minute intervals
	}
	autoMarketJSON, _ := json.Marshal(autoMarketReq)
	resp, err = http.Post(serverURL+"/symbol/createmarket", "application/json", bytes.NewBuffer(autoMarketJSON))
	if err != nil {
		log.Printf("Failed to create automatic market: %v", err)
	} else {
		if resp.StatusCode == 200 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			fmt.Printf("✓ Automatic market creation scheduled: %s\n", string(body))
			// Wait a bit to let automatic markets start
			time.Sleep(2 * time.Second)
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			log.Printf("Create automatic market returned status: %d, body: %s", resp.StatusCode, string(body))
		}
	}

	// Test 3: Subscribe to WebSocket for market updates
	fmt.Println("Test 3: Subscribing to WebSocket for market updates...")
	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8081/ws", nil)
	if err != nil {
		log.Printf("Failed to connect to WebSocket: %v, skipping WebSocket test", err)
		conn = nil
	} else {
		defer conn.Close()

		// Read initial connection message
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Failed to read initial message: %v, skipping WebSocket test", err)
			conn = nil
		} else {
			fmt.Printf("Initial WebSocket message: %s\n", string(msg))

			// Send subscribe message
			subscribeMsg := map[string]string{
				"type":   "subscribe",
				"symbol": marketSymbol,
			}
			subscribeJSON, _ := json.Marshal(subscribeMsg)
			if err := conn.WriteMessage(websocket.TextMessage, subscribeJSON); err != nil {
				log.Printf("Failed to send subscribe message: %v, skipping WebSocket test", err)
				conn = nil
			} else {
				// Read confirmation
				_, msg, err = conn.ReadMessage()
				if err != nil {
					log.Printf("Failed to read subscribe confirmation: %v, skipping WebSocket test", err)
					conn = nil
				} else {
					fmt.Printf("✓ WebSocket subscribed: %s\n", string(msg))

					// Listen for WebSocket messages during trading
					go func() {
						for {
							_, msg, err := conn.ReadMessage()
							if err != nil {
								log.Println("WebSocket read error:", err)
								return
							}
							fmt.Printf("WebSocket update: %s\n", string(msg))
						}
					}()
				}
			}
		}
	}

	// Test 4: Place random orders for users (multiple per user until they can't)
	fmt.Println("Test 4: Placing random orders...")
	for i := 0; i < numUsers; i++ {
		userID := userIDs[i]
		attempts := 0
		maxAttempts := 20
		for attempts < maxAttempts {
			attempts++
			orderType := "buy"
			if r.Float32() < 0.5 {
				orderType = "sell"
			}
			stockType := "yes"
			if r.Float32() < 0.5 {
				stockType = "no"
			}
			quantity := 1
			price := float64(10 + r.Intn(81)) // 10 to 90 USD

			orderProps := types.OrderProps{
				UserId:      userID,
				StockType:   stockType,
				Quantity:    float64(quantity),
				Price:       price,
				StockSymbol: marketSymbol,
			}

			var url string
			if orderType == "buy" {
				url = serverURL + "/order/buy"
			} else {
				url = serverURL + "/order/sell"
			}

			jsonData, _ := json.Marshal(orderProps)
			resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
			if err != nil {
				log.Printf("Failed to place order for user %s: %v", userID, err)
				break
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 {
				var orderResp map[string]interface{}
				json.Unmarshal(body, &orderResp)
				if status, ok := orderResp["status"].(bool); ok && status {
					fmt.Printf("✓ Order placed for user %s (%s %s %.0f x %d)\n", userID, orderType, stockType, price, quantity)
				} else {
					message, _ := orderResp["message"].(string)
					if message == "insufficient balance" || message == "user doesn't have the required quantity" {
						// Can't place more orders
						break
					} else {
						log.Printf("Order failed for user %s: %s", userID, message)
						break
					}
				}
			} else {
				log.Printf("Order failed for user %s: %d", userID, resp.StatusCode)
				break
			}
		}
	}

	// Wait for order processing
	time.Sleep(1 * time.Second)

	// Test 5: Get order book
	fmt.Println("Test 5: Getting order book...")
	resp, err = http.Get(serverURL + "/book/get/" + marketSymbol)
	if err != nil {
		log.Fatalf("Failed to get order book: %v", err)
	}
	if resp.StatusCode != 200 {
		log.Fatalf("Get order book failed with status: %d", resp.StatusCode)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("✓ Order book: %s\n", string(body))

	// Test 6: Check user balances and stocks
	fmt.Println("Test 6: Checking user balances and stocks...")
	for _, userID := range userIDs {
		// Get balance
		resp, err = http.Get(serverURL + "/balance/get/" + userID)
		if err != nil {
			log.Printf("Failed to get balance for %s: %v", userID, err)
			continue
		}
		if resp.StatusCode == 200 {
			body, _ = io.ReadAll(resp.Body)
			fmt.Printf("✓ Balance for %s: %s\n", userID, string(body))
		}
		resp.Body.Close()

		// Get stocks
		resp, err = http.Get(serverURL + "/balance/stocks/" + userID)
		if err != nil {
			log.Printf("Failed to get stocks for %s: %v", userID, err)
			continue
		}
		if resp.StatusCode == 200 {
			body, _ = io.ReadAll(resp.Body)
			fmt.Printf("✓ Stocks for %s: %s\n", userID, string(body))
		}
		resp.Body.Close()
	}

	// Test 7: End market randomly
	fmt.Println("Test 7: Ending market...")
	winningStock := "yes"
	if r.Float32() < 0.5 {
		winningStock = "no"
	}
	endReq := map[string]interface{}{
		"stockSymbol":  marketSymbol,
		"marketId":     marketSymbol,
		"winningStock": winningStock,
	}
	endJSON, _ := json.Marshal(endReq)
	resp, err = http.Post(serverURL+"/order/endmarket", "application/json", bytes.NewBuffer(endJSON))
	if err != nil {
		log.Printf("Failed to end market: %v", err)
	} else {
		if resp.StatusCode == 200 {
			fmt.Printf("✓ Market ended successfully with %s winning\n", winningStock)
		} else {
			body, _ = io.ReadAll(resp.Body)
			fmt.Printf("Market end response: %s\n", string(body))
		}
		resp.Body.Close()
	}

	// Wait for settlement
	time.Sleep(1 * time.Second)

	// Test 8: Check final balances
	fmt.Println("Test 8: Checking final balances...")
	for _, userID := range userIDs {
		resp, err = http.Get(serverURL + "/balance/get/" + userID)
		if err != nil {
			log.Printf("Failed to get final balance for %s: %v", userID, err)
			continue
		}
		if resp.StatusCode == 200 {
			body, _ = io.ReadAll(resp.Body)
			var balResp BalanceResp
			json.Unmarshal(body, &balResp)
			// Check that locked is approximately 0
			lockedDiff := math.Abs(balResp.Data.Balance.Locked)
			if lockedDiff < 1e-10 {
				fmt.Printf("✓ Final balance for %s: %.2f USD (locked: %.2f)\n", userID, balResp.Data.Balance.Balance, balResp.Data.Balance.Locked)
			} else {
				log.Printf("Final balance check failed for %s: locked %.2f", userID, balResp.Data.Balance.Locked)
			}
		}
		resp.Body.Close()
	}

	fmt.Println("All random integration tests completed! ✓")
}
