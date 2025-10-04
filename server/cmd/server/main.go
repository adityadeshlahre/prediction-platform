package main

import (
	"context"
	"encoding/json"

	"github.com/adityadeshlahre/probo-v1/server/routes/handler/order"
	"github.com/adityadeshlahre/probo-v1/server/routes/handler/user"
	"github.com/adityadeshlahre/probo-v1/server/server"
	"github.com/adityadeshlahre/probo-v1/shared/redis"
	"github.com/adityadeshlahre/probo-v1/shared/types"
	"github.com/joho/godotenv"
)

const DefaultContextTimeout = 30

func main() {
	godotenv.Load()
	redis.InitRedis()
	ctx := context.Background()
	subscriberClient := redis.GetRedisClient()
	responsePubsub := subscriberClient.Subscribe(ctx, "server-responses")

	go func() {
		for msg := range responsePubsub.Channel() {
			var resp types.IncomingMessage
			if err := json.Unmarshal([]byte(msg.Payload), &resp); err == nil {
				var user types.User
				if err := json.Unmarshal(resp.Data, &user); err == nil {
					if ch, ok := redis.ServerAwaitsForResponseMap[user.Id]; ok {
						ch <- msg.Payload
						delete(redis.ServerAwaitsForResponseMap, user.Id)
					}
				}
			}
			println("Received message in server:", msg.Payload)
		}
	}()

	e := server.NewServer()
	user.InitUserRoute(e)
	order.InitOrderRoutes(e)
	e.Logger.Fatal(e.Start(":8080"))
}
