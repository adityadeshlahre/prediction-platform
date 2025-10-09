package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	types "github.com/adityadeshlahre/probo-v1/shared/types"
	"github.com/gorilla/websocket"
)

const (
	serverURL = "http://localhost:8080"
	numUsers  = 1
)

func main() {
	fmt.Println("Starting comprehensive integration tests for probo-v1 application...")

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

	// Test 4: Place orders to test trading functionality
	fmt.Println("Test 4: Placing orders...")
	// Place some buy and sell orders for YES stock
	buyOrder1 := types.OrderProps{
		UserId:      userIDs[0],
		StockType:   "yes",
		Quantity:    10,
		Price:       0.6,
		StockSymbol: marketSymbol,
	}
	buyJSON1, _ := json.Marshal(buyOrder1)
	resp, err = http.Post(serverURL+"/order/buy", "application/json", bytes.NewBuffer(buyJSON1))
	if err != nil {
		log.Printf("Failed to place buy order 1: %v", err)
	} else {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			fmt.Println("✓ Buy order 1 placed")
		} else {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("Buy order 1 failed: %d, %s", resp.StatusCode, string(body))
		}
	}

	sellOrder1 := types.OrderProps{
		UserId:      userIDs[1],
		StockType:   "yes",
		Quantity:    10,
		Price:       0.6,
		StockSymbol: marketSymbol,
	}
	sellJSON1, _ := json.Marshal(sellOrder1)
	resp, err = http.Post(serverURL+"/order/sell", "application/json", bytes.NewBuffer(sellJSON1))
	if err != nil {
		log.Printf("Failed to place sell order 1: %v", err)
	} else {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			fmt.Println("✓ Sell order 1 placed (should match buy)")
		} else {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("Sell order 1 failed: %d, %s", resp.StatusCode, string(body))
		}
	}

	// Place orders for NO stock
	buyOrder2 := types.OrderProps{
		UserId:      userIDs[2],
		StockType:   "no",
		Quantity:    5,
		Price:       0.4,
		StockSymbol: marketSymbol,
	}
	buyJSON2, _ := json.Marshal(buyOrder2)
	resp, err = http.Post(serverURL+"/order/buy", "application/json", bytes.NewBuffer(buyJSON2))
	if err != nil {
		log.Printf("Failed to place buy order 2: %v", err)
	} else {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			fmt.Println("✓ Buy order 2 placed (NO stock)")
		} else {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("Buy order 2 failed: %d, %s", resp.StatusCode, string(body))
		}
	}

	sellOrder2 := types.OrderProps{
		UserId:      userIDs[3],
		StockType:   "no",
		Quantity:    5,
		Price:       0.4,
		StockSymbol: marketSymbol,
	}
	sellJSON2, _ := json.Marshal(sellOrder2)
	resp, err = http.Post(serverURL+"/order/sell", "application/json", bytes.NewBuffer(sellJSON2))
	if err != nil {
		log.Printf("Failed to place sell order 2: %v", err)
	} else {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			fmt.Println("✓ Sell order 2 placed (NO stock, should match)")
		} else {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("Sell order 2 failed: %d, %s", resp.StatusCode, string(body))
		}
	}

	// Place a non-matching order
	buyOrder3 := types.OrderProps{
		UserId:      userIDs[4],
		StockType:   "yes",
		Quantity:    20,
		Price:       0.7,
		StockSymbol: marketSymbol,
	}
	buyJSON3, _ := json.Marshal(buyOrder3)
	resp, err = http.Post(serverURL+"/order/buy", "application/json", bytes.NewBuffer(buyJSON3))
	if err != nil {
		log.Printf("Failed to place buy order 3: %v", err)
	} else {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			fmt.Println("✓ Buy order 3 placed (non-matching)")
		} else {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("Buy order 3 failed: %d, %s", resp.StatusCode, string(body))
		}
	}

	// Wait for order processing
	time.Sleep(1 * time.Second)

	// Verify balances after order placement
	fmt.Println("Verifying balances after order placement...")
	// user0: bought 10 YES at 0.6, matched, so -6 USD, locked 0
	resp, err = http.Get(serverURL + "/balance/get/" + userIDs[0])
	if err == nil && resp.StatusCode == 200 {
		body, _ := io.ReadAll(resp.Body)
		var bal types.Balance
		json.Unmarshal(body, &bal)
		if bal.Balance == 9994 && bal.Locked == 0 {
			fmt.Println("✓ Balance correct for user0 (matched buy)")
		} else {
			log.Printf("Balance check failed for user0: %+v", bal)
		}
	}
	resp.Body.Close()

	// user1: sold 10 YES at 0.6, matched, +6 USD, locked 0
	resp, err = http.Get(serverURL + "/balance/get/" + userIDs[1])
	if err == nil && resp.StatusCode == 200 {
		body, _ := io.ReadAll(resp.Body)
		var bal types.Balance
		json.Unmarshal(body, &bal)
		if bal.Balance == 10006 && bal.Locked == 0 {
			fmt.Println("✓ Balance correct for user1 (matched sell)")
		} else {
			log.Printf("Balance check failed for user1: %+v", bal)
		}
	}
	resp.Body.Close()

	// user4: bought 20 YES at 0.7, not matched, -14 USD locked
	resp, err = http.Get(serverURL + "/balance/get/" + userIDs[4])
	if err == nil && resp.StatusCode == 200 {
		body, _ := io.ReadAll(resp.Body)
		var bal types.Balance
		json.Unmarshal(body, &bal)
		if bal.Balance == 9986 && bal.Locked == 14 {
			fmt.Println("✓ Balance correct for user4 (reverted buy)")
		} else {
			log.Printf("Balance check failed for user4: %+v", bal)
		}
	}
	resp.Body.Close()

	// Test 5: Get order book
	fmt.Println("Test 5: Getting order book...")
	resp, err = http.Get(serverURL + "/book/get/" + marketSymbol)
	if err != nil {
		log.Fatalf("Failed to get final order book: %v", err)
	}
	if resp.StatusCode != 200 {
		log.Fatalf("Get final order book failed with status: %d", resp.StatusCode)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("✓ Final order book: %s\n", string(body))

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

	// Test 7: End market (manual trigger)
	fmt.Println("Test 7: Ending market...")
	endReq := map[string]interface{}{
		"stockSymbol":  marketSymbol,
		"marketId":     marketSymbol,
		"winningStock": "yes",
	}
	endJSON, _ := json.Marshal(endReq)
	resp, err = http.Post(serverURL+"/order/endmarket", "application/json", bytes.NewBuffer(endJSON))
	if err != nil {
		log.Printf("Failed to end market: %v", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			fmt.Println("✓ Market ended successfully")
		} else {
			body, _ = io.ReadAll(resp.Body)
			fmt.Printf("Market end response: %s\n", string(body))
		}
	}

	// Wait for settlement
	time.Sleep(1 * time.Second)

	// Verify final balances after market settlement
	fmt.Println("Verifying final balances after market settlement...")
	expectedBalances := map[string]float64{
		userIDs[0]: 19994, // 10000 -6 +10000
		userIDs[1]: 10006, // 10000 +6
		userIDs[2]: 9998,  // 10000 -2
		userIDs[3]: 10002, // 10000 +2
		userIDs[4]: 9986,  // 10000 -14
		userIDs[5]: 10000, // no activity
	}
	for _, userID := range userIDs {
		resp, err = http.Get(serverURL + "/balance/get/" + userID)
		if err != nil {
			log.Printf("Failed to get final balance for %s: %v", userID, err)
			continue
		}
		if resp.StatusCode == 200 {
			body, _ := io.ReadAll(resp.Body)
			var bal types.Balance
			json.Unmarshal(body, &bal)
			if bal.Balance == expectedBalances[userID] && bal.Locked == 0 {
				fmt.Printf("✓ Final balance correct for %s: %.0f USD\n", userID, bal.Balance)
			} else {
				log.Printf("Final balance check failed for %s: %+v, expected %.0f", userID, bal, expectedBalances[userID])
			}
		}
		resp.Body.Close()
	}

	fmt.Println("All comprehensive integration tests completed! ✓")
}
