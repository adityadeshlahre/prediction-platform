package user

import (
	"encoding/json"

	"github.com/adityadeshlahre/probo-v1/shared/redis"
	"github.com/adityadeshlahre/probo-v1/shared/types"
	"github.com/labstack/echo/v4"
)

var router *echo.Echo

func InitUserRoute(e *echo.Echo) {
	router = e
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
	correlationId := id
	user := types.User{Id: id}
	data, _ := json.Marshal(user)
	msg := types.IncomingMessage{
		Type:          "USER",
		Data:          data,
	}
	msgBytes, _ := json.Marshal(msg)
	client := redis.GetRedisClient()
	err := client.LPush(c.Request().Context(), "httptoengine", msgBytes).Err()
	if err != nil {
		return c.String(500, "Failed to send message")
	}
	// Await response
	ch := make(chan string, 1)
	redis.ServerAwaitsForResponseMap[correlationId] = ch
	<-ch
	return c.String(200, "User "+id+" created")
}
