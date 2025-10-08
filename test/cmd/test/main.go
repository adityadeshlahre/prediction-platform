package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	types "github.com/adityadeshlahre/probo-v1/shared/types"
	"github.com/gorilla/websocket"
)

const (
	serverURL = "http://localhost:8080"
	numUsers  = 6
)

func main() {
	fmt.Println("Starting comprehensive integration tests for probo-v1 application...")

	// Wait for services to be ready
	time.Sleep(3 * time.Second)

	// Test 1: Create multiple users
	fmt.Println("Test 1: Creating multiple users...")
	userIDs := make([]string, numUsers)
	for i := 0; i < numUsers; i++ {
		userID := fmt.Sprintf("testuser%d", i+1)
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

	// Test 2: Create a market
	fmt.Println("Test 2: Creating a prediction market...")
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
		log.Fatalf("Failed to create market: %v", err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		log.Fatalf("Create market failed with status: %d, body: %s", resp.StatusCode, string(body))
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
	fmt.Printf("✓ Market created successfully with symbol: %s\n", marketSymbol)

	// Test 3: Subscribe to WebSocket for market updates
	fmt.Println("Test 3: Subscribing to WebSocket for market updates...")
	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8081/ws", nil)
	if err != nil {
		log.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Read initial connection message
	_, msg, err := conn.ReadMessage()
	if err != nil {
		log.Fatalf("Failed to read initial message: %v", err)
	}
	fmt.Printf("Initial WebSocket message: %s\n", string(msg))

	// Send subscribe message
	subscribeMsg := map[string]string{
		"type":   "subscribe",
		"symbol": marketSymbol,
	}
	subscribeJSON, _ := json.Marshal(subscribeMsg)
	if err := conn.WriteMessage(websocket.TextMessage, subscribeJSON); err != nil {
		log.Fatalf("Failed to send subscribe message: %v", err)
	}

	// Read confirmation
	_, msg, err = conn.ReadMessage()
	if err != nil {
		log.Fatalf("Failed to read subscribe confirmation: %v", err)
	}
	fmt.Printf("✓ WebSocket subscribed: %s\n", string(msg))

	// Test 4: Multi-user trading
	fmt.Println("Test 4: Starting multi-user trading simulation...")
	var wg sync.WaitGroup
	orderCount := 10

	for i := 0; i < orderCount; i++ {
		wg.Add(1)
		go func(orderID int) {
			defer wg.Done()
			userID := userIDs[orderID%numUsers]
			stockType := "yes"
			if orderID%2 == 0 {
				stockType = "no"
			}
			order := types.OrderProps{
				UserId:      userID,
				StockSymbol: marketSymbol,
				Quantity:    float64(10 + orderID),
				Price:       float64(50 + orderID*1), // Varying prices
				StockType:   stockType,
			}
			orderJSON, _ := json.Marshal(order)
			var endpoint string
			if orderID%2 == 0 {
				endpoint = "/order/buy"
			} else {
				endpoint = "/order/sell"
			}
			resp, err := http.Post(serverURL+endpoint, "application/json", bytes.NewBuffer(orderJSON))
			if err != nil {
				log.Printf("Failed to place order %d: %v", orderID, err)
				return
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 200 {
				log.Printf("Order %d failed with status: %d, body: %s", orderID, resp.StatusCode, string(body))
				return
			}
			var responseData map[string]interface{}
			if err := json.Unmarshal(body, &responseData); err != nil {
				log.Printf("Failed to parse response for order %d: %v", orderID, err)
				return
			}
			if errorMsg, exists := responseData["error"]; exists {
				log.Printf("Order %d failed: %s", orderID, errorMsg)
			} else {
				fmt.Printf("✓ Order %d placed by %s\n", orderID, userID)
			}
		}(i)
	}

	wg.Wait()
	fmt.Println("✓ All orders placed")

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

	// Wait a bit for updates
	time.Sleep(2 * time.Second)

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

	fmt.Println("All comprehensive integration tests completed! ✓")
}
