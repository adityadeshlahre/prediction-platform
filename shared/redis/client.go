package redis

import (
	"context"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var redisOptions *redis.Options
var ServerAwaitsForResponseMap = make(map[string]chan string)

// InitRedis initializes the Redis client configuration
func InitRedis() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisPassword := os.Getenv("REDIS_PASSWORD")

	redisDBStr := os.Getenv("REDIS_DB")
	redisDB := 0
	if redisDBStr != "" {
		if db, err := strconv.Atoi(redisDBStr); err == nil {
			redisDB = db
		}
	}

	redisOptions = &redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	}

	// Test connection with a temporary client
	testClient := redis.NewClient(redisOptions)
	ctx := context.Background()
	_, err = testClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	testClient.Close()

	log.Println("Connected to Redis")
}

// GetRedisClient returns a new Redis client instance
func GetRedisClient() *redis.Client {
	if redisOptions == nil {
		log.Fatal("Redis options not initialized. Call InitRedis() first.")
	}
	return redis.NewClient(redisOptions)
}
