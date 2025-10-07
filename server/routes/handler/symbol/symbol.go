package symbol

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	sharedRedis "github.com/adityadeshlahre/probo-v1/shared/redis"
	types "github.com/adityadeshlahre/probo-v1/shared/types"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

var router *echo.Echo

var serverToEngineClient *redis.Client

var automaticMarketsAllowed = []string{"bitcoin", "ethereum"}

func InitSymbolRoutes(e *echo.Echo, client *redis.Client) {
	router = e
	serverToEngineClient = client
	symbolRoutes()
}

func symbolRoutes() {
	symbolGroup := router.Group("/symbol")
	{
		symbolGroup.POST("/createmarket", createMarket)
	}
}

func getCurrentMarketPrice(stockSymbol string) (float64, error) {
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd", stockSymbol)
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var data map[string]map[string]float64
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, err
	}

	if symbolData, ok := data[stockSymbol]; ok {
		if price, ok := symbolData["usd"]; ok {
			return price, nil
		}
	}

	return 0, fmt.Errorf("price not found for symbol: %s", stockSymbol)
}

func createMarketCondition(stockSymbol string) (float64, error) {
	price, err := getCurrentMarketPrice(stockSymbol)
	if err != nil {
		return 0, err
	}

	// Random multiplier: +1 or -1
	multiplier := float64(1)
	if rand.Float64() > 0.5 {
		multiplier = -1
	}

	couldBePrice := multiplier*price*0.02 + price
	return couldBePrice, nil
}

func createMarket(c echo.Context) error {
	var req types.CreateMarket
	if err := c.Bind(&req); err != nil {
		return c.String(400, "Invalid request body")
	}

	endTime := time.Now().UnixMilli() + req.EndAfterTime

	if req.MarketType == "automatic" && req.SourceOfTruth == "automatic" {
		if contains(automaticMarketsAllowed, req.Symbol) {
			repeatInterval := req.RepeatEventTime
			if repeatInterval == 0 {
				repeatInterval = 60000 // Default 1 minute
			}

			go func() {
				ticker := time.NewTicker(time.Duration(repeatInterval) * time.Millisecond)
				defer ticker.Stop()

				for range ticker.C {
					currentTime := time.Now().UnixMilli()
					if currentTime > endTime {
						break
					}

					couldBePrice, err := createMarketCondition(req.Symbol)
					if err != nil {
						fmt.Printf("Error creating market condition: %v\n", err)
						continue
					}

					stockUniqueSymbol := fmt.Sprintf("%s-%d", req.Symbol, time.Now().UnixMilli())

					marketData := map[string]interface{}{
						"symbol":    stockUniqueSymbol,
						"price":     couldBePrice,
						"heading":   req.Heading,
						"eventType": req.EventType,
						"type":      req.MarketType,
					}

					data, _ := json.Marshal(marketData)
					msg := types.IncomingMessage{
						Type: string(types.CREATE_MARKET),
						Data: data,
					}
					msgBytes, _ := json.Marshal(msg)

					err = serverToEngineClient.LPush(c.Request().Context(), types.HTTP_TO_ENGINE, msgBytes).Err()
					if err != nil {
						fmt.Printf("Error sending create market message: %v\n", err)
						continue
					}

					// Wait for response
					ch := make(chan string, 1)
					sharedRedis.ServerAwaitsForResponseMap["create_market_"+stockUniqueSymbol] = ch
					response := <-ch

					var respData map[string]interface{}
					json.Unmarshal([]byte(response), &respData)

					marketId := ""
					for key := range respData {
						if key != "status" {
							marketId = key
							break
						}
					}

					// Schedule market end
					go func(marketId, stockUniqueSymbol string, couldBePrice float64) {
						time.Sleep(time.Duration(req.EndsIn) * time.Millisecond)

						currentPrice, err := getCurrentMarketPrice(req.Symbol)
						if err != nil {
							fmt.Printf("Error getting current price: %v\n", err)
							return
						}

						winningStock := "no"
						if currentPrice > couldBePrice {
							winningStock = "yes"
						}

						endData := map[string]interface{}{
							"symbol":       stockUniqueSymbol,
							"marketId":     marketId,
							"winningStock": winningStock,
						}

						data, _ := json.Marshal(endData)
						msg := types.IncomingMessage{
							Type: string(types.END_MARKET),
							Data: data,
						}
						msgBytes, _ := json.Marshal(msg)

						serverToEngineClient.LPush(c.Request().Context(), types.HTTP_TO_ENGINE, msgBytes).Err()
					}(marketId, stockUniqueSymbol, couldBePrice)
				}
			}()

			return c.String(200, "Market creation scheduled")
		} else {
			return c.String(400, "Stock symbol not allowed for automatic markets")
		}
	} else {
		return c.String(400, "Invalid type or sourceOfTruth")
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
