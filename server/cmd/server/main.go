package main

import (
	"context"
	"encoding/json"

	"github.com/adityadeshlahre/probo-v1/server/routes/handler/order"
	"github.com/adityadeshlahre/probo-v1/server/routes/handler/user"
	"github.com/adityadeshlahre/probo-v1/server/server"
	sharedRedis "github.com/adityadeshlahre/probo-v1/shared/redis"
	"github.com/adityadeshlahre/probo-v1/shared/types"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

const DefaultContextTimeout = 30

var ctx = context.Background()

var serverToEngineProducerClient *redis.Client

var serverToEnginePubSubClient *redis.Client

func main() {
	godotenv.Load()
	sharedRedis.InitRedis()
	serverToEngineProducerClient = sharedRedis.GetRedisClient()
	serverToEnginePubSubClient = sharedRedis.GetRedisClient()
	responsePubsub := serverToEnginePubSubClient.Subscribe(ctx, "server-responses")

	go func() {
		for msg := range responsePubsub.Channel() {
			var resp types.IncomingMessage
			if err := json.Unmarshal([]byte(msg.Payload), &resp); err == nil {
				var user types.User
				if err := json.Unmarshal(resp.Data, &user); err == nil {
					if ch, ok := sharedRedis.ServerAwaitsForResponseMap[user.Id]; ok {
						ch <- msg.Payload
						delete(sharedRedis.ServerAwaitsForResponseMap, user.Id)
					}
				}
			}
			println("Received message in server:", msg.Payload)
		}
	}()

	e := server.NewServer()
	user.InitUserRoute(e, serverToEngineProducerClient)
	order.InitOrderRoutes(e, serverToEngineProducerClient)
	e.Logger.Fatal(e.Start(":8080"))
}
