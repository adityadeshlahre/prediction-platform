package balance

import (
	"encoding/json"

	sharedRedis "github.com/adityadeshlahre/probo-v1/shared/redis"
	types "github.com/adityadeshlahre/probo-v1/shared/types"
	"github.com/labstack/echo/v4"

	// gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/redis/go-redis/v9"
)

var router *echo.Echo
var serverToEngineClient *redis.Client

func InitBalanceRoutes(e *echo.Echo, client *redis.Client) {
	router = e
	serverToEngineClient = client
	balanceRoutes()
}

func balanceRoutes() {
	balanceGroup := router.Group("/balance")
	{
		balanceGroup.GET("/get", getBalance)
		balanceGroup.GET("/stocks", getStocks)
		balanceGroup.GET("/get/:id", getBalanceById)
		balanceGroup.GET("/stocks/:id", getStocksById)
	}
}

func getBalance(c echo.Context) error {
	balance := types.Balance{}
	data, _ := json.Marshal(balance)
	msg := types.IncomingMessage{
		Type: types.GET_BALANCE,
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	err := serverToEngineClient.LPush(c.Request().Context(), types.HTTP_TO_ENGINE, msgBytes).Err()
	if err != nil {
		return c.String(500, "Failed to send message")
	}
	// Await response
	ch := make(chan string, 1)
	sharedRedis.ServerAwaitsForResponseMap["get_balance"] = ch
	response := <-ch
	return c.String(200, response)
}

func getStocks(c echo.Context) error {
	balance := types.Balance{}
	data, _ := json.Marshal(balance)
	msg := types.IncomingMessage{
		Type: string(types.GET_STOCKS),
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	err := serverToEngineClient.LPush(c.Request().Context(), types.HTTP_TO_ENGINE, msgBytes).Err()
	if err != nil {
		return c.String(500, "Failed to send message")
	}
	// Await response
	ch := make(chan string, 1)
	sharedRedis.ServerAwaitsForResponseMap["get_stocks"] = ch
	response := <-ch
	return c.String(200, response)
}

func getBalanceById(c echo.Context) error {
	id := c.Param("id")
	balance := types.Balance{UserId: id}
	data, _ := json.Marshal(balance)
	msg := types.IncomingMessage{
		Type: "GET_BALANCE",
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	err := serverToEngineClient.LPush(c.Request().Context(), "HTTP_TO_ENGINE", msgBytes).Err()
	if err != nil {
		return c.String(500, "Failed to send message")
	}
	// Await response
	ch := make(chan string, 1)
	sharedRedis.ServerAwaitsForResponseMap[id] = ch
	response := <-ch
	return c.String(200, response)
}

func getStocksById(c echo.Context) error {
	id := c.Param("id")
	balance := types.Balance{UserId: id}
	data, _ := json.Marshal(balance)
	msg := types.IncomingMessage{
		Type: "GET_STOCKS",
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	err := serverToEngineClient.LPush(c.Request().Context(), "HTTP_TO_ENGINE", msgBytes).Err()
	if err != nil {
		return c.String(500, "Failed to send message")
	}
	// Await response
	ch := make(chan string, 1)
	sharedRedis.ServerAwaitsForResponseMap[id] = ch
	response := <-ch
	return c.String(200, response)
}
