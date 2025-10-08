package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/adityadeshlahre/probo-v1/server/routes/handler/book"
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

var serverResponseQueueClient *redis.Client

func main() {
	godotenv.Load()
	sharedRedis.InitRedis()
	serverToEngineQueueClient = sharedRedis.GetRedisClient()
	serverResponseQueueClient = sharedRedis.GetRedisClient()

	go func() {
		for {
			res, err := serverResponseQueueClient.BRPop(ctx, 0, "SERVER_RESPONSES_QUEUE").Result()
			if err != nil {
				log.Println("Error popping from response queue:", err)
				continue
			}
			message := res[1]
			var resp types.IncomingMessage
			if err := json.Unmarshal([]byte(message), &resp); err == nil {
				switch resp.Type {
				case types.USER:
					var user types.User
					if err := json.Unmarshal(resp.Data, &user); err == nil {
						if ch, ok := sharedRedis.ServerAwaitsForResponseMap[user.Id]; ok {
							ch <- message
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
									ch <- message
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
								ch <- message
								delete(sharedRedis.ServerAwaitsForResponseMap, userId)
							}
						}
					}
				case string(types.GET_ORDER_BOOK):
					var data map[string]interface{}
					if err := json.Unmarshal(resp.Data, &data); err == nil {
						if symbol, ok := data["symbol"].(string); ok {
							chKey := "get_order_book_" + symbol
							if ch, ok := sharedRedis.ServerAwaitsForResponseMap[chKey]; ok {
								ch <- message
								delete(sharedRedis.ServerAwaitsForResponseMap, chKey)
							}
						}
					}
				}
			}
			println("Received message in server:", message)
		}
	}()

	e := server.NewServer()
	user.InitUserRoute(e, serverToEngineQueueClient)
	order.InitOrderRoutes(e, serverToEngineQueueClient)
	symbol.InitSymbolRoutes(e, serverToEngineQueueClient)
	book.InitBookRoutes(e, serverToEngineQueueClient)
	e.Logger.Fatal(e.Start(":8080"))
}
