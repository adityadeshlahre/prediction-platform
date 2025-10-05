package symbol

import (
	// sharedRedis "github.com/adityadeshlahre/probo-v1/shared/redis"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

var router *echo.Echo

var serverToEngineClient *redis.Client

func InitSymbolRoutes(e *echo.Echo, client *redis.Client) {
	router = e
	serverToEngineClient = client
	// symbolRoutes()
}

// func symbolRoutes() {
// 	symbolGroup := router.Group("/symbol")
// 	{
// 		symbolGroup.GET("/list", listSymbols)
// 	}
// }

// func listSymbols(c echo.Context) error {
// 	symbol := struct{}{}
// 	msg := sharedRedis.CreateMessage("LIST_SYMBOLS", symbol)
// 	msgBytes, _ := sharedRedis.MarshalMessage(msg)
// 	err := serverToEngineClient.LPush(c.Request().Context(), "httptoengine", msgBytes).Err()
// 	if err != nil {
// 		return c.String(500, "Failed to send message")
// 	}
// 	// Await response
// 	ch := make(chan string, 1)
// 	sharedRedis.ServerAwaitsForResponseMap["list_symbols"] = ch
// 	response := <-ch
// 	return c.String(200, response)
// }
