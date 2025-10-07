package order

import (
	"encoding/json"

	sharedRedis "github.com/adityadeshlahre/probo-v1/shared/redis"
	types "github.com/adityadeshlahre/probo-v1/shared/types"
	"github.com/labstack/echo/v4"
	gonanoid "github.com/matoous/go-nanoid/v2"
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
	}
}

func placeBuyOrder(c echo.Context) error {
	id, err := gonanoid.New()
	if err != nil {
		return c.String(500, "Failed to generate order ID")
	}
	order := types.Order{Id: id, OrderType: "BUY"}
	data, _ := json.Marshal(order)
	msg := types.IncomingMessage{
		Type: "ORDER",
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	err = serverToEngineQueueClient.LPush(c.Request().Context(), "HTTP_TO_ENGINE", msgBytes).Err()
	if err != nil {
		return c.String(500, "Failed to send message")
	}
	// Await response
	ch := make(chan string, 1)
	sharedRedis.ServerAwaitsForResponseMap[id] = ch
	<-ch
	return c.String(200, "Buy order placed")
}

func placeSellOrder(c echo.Context) error {
	id, err := gonanoid.New()
	if err != nil {
		return c.String(500, "Failed to generate order ID")
	}
	order := types.Order{Id: id, OrderType: "SELL"}
	data, _ := json.Marshal(order)
	msg := types.IncomingMessage{
		Type: "ORDER",
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	err = serverToEngineQueueClient.LPush(c.Request().Context(), "HTTP_TO_ENGINE", msgBytes).Err()
	if err != nil {
		return c.String(500, "Failed to send message")
	}
	// Await response
	ch := make(chan string, 1)
	sharedRedis.ServerAwaitsForResponseMap[id] = ch
	<-ch
	return c.String(200, "Sell order placed")
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
		Type: "CANCEL_ORDER",
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	err := serverToEngineQueueClient.LPush(c.Request().Context(), "HTTP_TO_ENGINE", msgBytes).Err()
	if err != nil {
		return c.String(500, "Failed to send message")
	}
	// Await response
	ch := make(chan string, 1)
	sharedRedis.ServerAwaitsForResponseMap[req.OrderId] = ch
	<-ch
	return c.String(200, "Order cancelled")
}
