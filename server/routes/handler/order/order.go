package order

import (
	"encoding/json"

	sharedRedis "github.com/adityadeshlahre/probo-v1/shared/redis"
	types "github.com/adityadeshlahre/probo-v1/shared/types"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

var router *echo.Echo
var serverToEngineQueueClient *redis.Client

func InitOrderRoutes(e *echo.Echo, client *redis.Client) {
	router = e
	serverToEngineQueueClient = client
	orderRoutes()
}

func orderRoutes() {
	orderGroup := router.Group("/order")
	{
		orderGroup.POST("/buy", placeBuyOrder)
		orderGroup.POST("/sell", placeSellOrder)
		orderGroup.POST("/cancel", cancelOrder)
		orderGroup.POST("/endmarket", endMarket)
	}
}

func placeBuyOrder(c echo.Context) error {
	var orderProps types.OrderProps
	if err := c.Bind(&orderProps); err != nil {
		return c.JSON(400, map[string]string{"error": "Invalid order data"})
	}

	data, _ := json.Marshal(orderProps)
	msg := types.IncomingMessage{
		Type: string(types.BUY_ORDER),
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	err := serverToEngineQueueClient.LPush(c.Request().Context(), types.HTTP_TO_ENGINE, msgBytes).Err()
	if err != nil {
		return c.JSON(500, map[string]string{"error": "Failed to send message"})
	}

	// Await response
	ch := make(chan string, 1)
	sharedRedis.ServerAwaitsForResponseMap[orderProps.UserId] = ch
	response := <-ch

	var resp types.IncomingMessage
	if err := json.Unmarshal([]byte(response), &resp); err == nil && resp.Type == string(types.BUY_ORDER) {
		var data map[string]interface{}
		json.Unmarshal(resp.Data, &data)
		return c.JSON(200, data)
	}

	return c.JSON(500, map[string]string{"error": "Failed to place buy order"})
}

func placeSellOrder(c echo.Context) error {
	var orderProps types.OrderProps
	if err := c.Bind(&orderProps); err != nil {
		return c.JSON(400, map[string]string{"error": "Invalid order data"})
	}

	data, _ := json.Marshal(orderProps)
	msg := types.IncomingMessage{
		Type: string(types.SELL_ORDER),
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	err := serverToEngineQueueClient.LPush(c.Request().Context(), types.HTTP_TO_ENGINE, msgBytes).Err()
	if err != nil {
		return c.JSON(500, map[string]string{"error": "Failed to send message"})
	}

	// Await response
	ch := make(chan string, 1)
	sharedRedis.ServerAwaitsForResponseMap[orderProps.UserId] = ch
	response := <-ch

	var resp types.IncomingMessage
	if err := json.Unmarshal([]byte(response), &resp); err == nil && resp.Type == string(types.SELL_ORDER) {
		var data map[string]interface{}
		json.Unmarshal(resp.Data, &data)
		return c.JSON(200, data)
	}

	return c.JSON(500, map[string]string{"error": "Failed to place sell order"})
}

func cancelOrder(c echo.Context) error {
	var req struct {
		OrderId string `json:"orderId"`
	}
	if err := c.Bind(&req); err != nil {
		return c.String(400, "Invalid request")
	}
	data, _ := json.Marshal(req)
	msg := types.IncomingMessage{
		Type: types.CANCEL_ORDER,
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	err := serverToEngineQueueClient.LPush(c.Request().Context(), types.HTTP_TO_ENGINE, msgBytes).Err()
	if err != nil {
		return c.String(500, "Failed to send message")
	}
	// Await response
	ch := make(chan string, 1)
	sharedRedis.ServerAwaitsForResponseMap[req.OrderId] = ch
	<-ch
	return c.String(200, "Order cancelled")
}

func endMarket(c echo.Context) error {
	var req struct {
		StockSymbol  string `json:"stockSymbol"`
		MarketId     string `json:"marketId"`
		WinningStock string `json:"winningStock"`
	}
	if err := c.Bind(&req); err != nil {
		return c.String(400, "Invalid request")
	}
	data, _ := json.Marshal(req)
	msg := types.IncomingMessage{
		Type: string(types.END_MARKET),
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	err := serverToEngineQueueClient.LPush(c.Request().Context(), types.HTTP_TO_ENGINE, msgBytes).Err()
	if err != nil {
		return c.String(500, "Failed to send message")
	}
	// For end market, perhaps no response needed, or await
	return c.String(200, "Market end initiated")
}
