package main

import (
	"context"

	"github.com/adityadeshlahre/probo-v1/server/server"
	"github.com/adityadeshlahre/probo-v1/shared/redis"
	"github.com/joho/godotenv"
)

const DefaultContextTimeout = 30

func main() {
	godotenv.Load()
	redis.InitRedis()
	ctx := context.Background()
	client := redis.GetRedisClient()
	subscriberClient := redis.GetRedisClient()
	subscriberPubsub := subscriberClient.Subscribe(ctx, "httptoengine")
	err := client.Publish(ctx, "httptoengine", "Hello, Redis!").Err()
	if err != nil {
		panic(err)
	}

	go func() {
		for msg := range subscriberPubsub.Channel() {
			println("Received message:", msg.Payload)
		}
	}()

	e := server.NewServer()
	e.Logger.Fatal(e.Start(":8080"))
}
