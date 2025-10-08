package main

import (
	"context"
	"encoding/json"

	"github.com/adityadeshlahre/probo-v1/server/routes/handler/order"
	"github.com/adityadeshlahre/probo-v1/server/routes/handler/symbol"
	"github.com/adityadeshlahre/probo-v1/server/routes/handler/user"
	"github.com/adityadeshlahre/probo-v1/server/server"
	sharedRedis "github.com/adityadeshlahre/probo-v1/shared/redis"
	types "github.com/adityadeshlahre/probo-v1/shared/types"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

const DefaultContextTimeout = 30

var ctx = context.Background()

var serverToEngineQueueClient *redis.Client

var serverResponseSubscriber *redis.Client

func main() {
	godotenv.Load()
	sharedRedis.InitRedis()
	serverToEngineQueueClient = sharedRedis.GetRedisClient()
	serverResponseSubscriber = sharedRedis.GetRedisClient()
	responsePubsub := serverResponseSubscriber.Subscribe(ctx, types.SERVER_RESPONSES)

	go func() {
		for msg := range responsePubsub.Channel() {
			var resp types.IncomingMessage
			if err := json.Unmarshal([]byte(msg.Payload), &resp); err == nil {
				switch resp.Type {
				case types.USER:
					var user types.User
					if err := json.Unmarshal(resp.Data, &user); err == nil {
						if ch, ok := sharedRedis.ServerAwaitsForResponseMap[user.Id]; ok {
							ch <- msg.Payload
							delete(sharedRedis.ServerAwaitsForResponseMap, user.Id)
						}
					}
				case string(types.CREATE_MARKET):
					var data map[string]interface{}
					if err := json.Unmarshal(resp.Data, &data); err == nil {
						for key := range data {
							if key != "status" {
								chKey := "create_market_" + key
								if ch, ok := sharedRedis.ServerAwaitsForResponseMap[chKey]; ok {
									ch <- msg.Payload
									delete(sharedRedis.ServerAwaitsForResponseMap, chKey)
								}
								break
							}
						}
					}
				case string(types.BUY_ORDER), string(types.SELL_ORDER):
					var data map[string]interface{}
					if err := json.Unmarshal(resp.Data, &data); err == nil {
						if userId, ok := data["userId"].(string); ok {
							if ch, ok := sharedRedis.ServerAwaitsForResponseMap[userId]; ok {
								ch <- msg.Payload
								delete(sharedRedis.ServerAwaitsForResponseMap, userId)
							}
						}
					}
				}
			}
			println("Received message in server:", msg.Payload)
		}
	}()

	e := server.NewServer()
	user.InitUserRoute(e, serverToEngineQueueClient)
	order.InitOrderRoutes(e, serverToEngineQueueClient)
	symbol.InitSymbolRoutes(e, serverToEngineQueueClient)
	e.Logger.Fatal(e.Start(":8080"))
}
