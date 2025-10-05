package user

import (
	"encoding/json"

	sharedRedis "github.com/adityadeshlahre/probo-v1/shared/redis"
	"github.com/adityadeshlahre/probo-v1/shared/types"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

var router *echo.Echo
var serverToEngineQueueClient *redis.Client

func InitUserRoute(e *echo.Echo, client *redis.Client) {
	router = e
	serverToEngineQueueClient = client
	userRoutes()
}

func userRoutes() {
	userGroup := router.Group("/user")
	{
		userGroup.POST("/:id", createUser)
	}
}

func createUser(c echo.Context) error {
	id := c.Param("id")
	user := types.User{Id: id}
	data, _ := json.Marshal(user)
	msg := types.IncomingMessage{
		Type: "USER",
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	err := serverToEngineQueueClient.LPush(c.Request().Context(), "HTTP_TO_ENGINE", msgBytes).Err()
	if err != nil {
		return c.String(500, "Failed to send message")
	}
	// Await response
	ch := make(chan string, 1)
	sharedRedis.ServerAwaitsForResponseMap[id] = ch
	<-ch
	return c.String(200, "User "+id+" created")
}
