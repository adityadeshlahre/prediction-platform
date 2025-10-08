package book

import (
	"encoding/json"

	sharedRedis "github.com/adityadeshlahre/probo-v1/shared/redis"
	types "github.com/adityadeshlahre/probo-v1/shared/types"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

var router *echo.Echo
var serverToEngineClient *redis.Client

func InitBookRoutes(e *echo.Echo, client *redis.Client) {
	router = e
	serverToEngineClient = client
	bookRoutes()
}

func bookRoutes() {
	bookGroup := router.Group("/book")
	{
		bookGroup.GET("/get", getAllOrderBooks)
		bookGroup.GET("/get/:symbol", getOrderBookBySymbol)
	}
}

func getAllOrderBooks(c echo.Context) error {
	symbol := struct{}{}
	data, _ := json.Marshal(symbol)
	msg := types.IncomingMessage{
		Type: string(types.GET_ORDER_BOOK),
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	err := serverToEngineClient.LPush(c.Request().Context(), types.HTTP_TO_ENGINE, msgBytes).Err()
	if err != nil {
		return c.String(500, "Failed to send message")
	}
	// Await response
	ch := make(chan string, 1)
	sharedRedis.ServerAwaitsForResponseMap["get_all_order_books"] = ch
	response := <-ch
	return c.String(200, response)
}

func getOrderBookBySymbol(c echo.Context) error {
	symbol := c.Param("symbol")
	data, _ := json.Marshal(struct {
		Symbol string `json:"symbol"`
	}{
		Symbol: symbol,
	})
	msg := types.IncomingMessage{
		Type: string(types.GET_ORDER_BOOK),
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	err := serverToEngineClient.LPush(c.Request().Context(), types.HTTP_TO_ENGINE, msgBytes).Err()
	if err != nil {
		return c.String(500, "Failed to send message")
	}
	// Await response
	ch := make(chan string, 1)
	sharedRedis.ServerAwaitsForResponseMap["get_order_book_"+symbol] = ch
	response := <-ch
	return c.String(200, response)
}
