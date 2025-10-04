package main

import (
	"github.com/adityadeshlahre/probo-v1/shared/redis"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type CSD struct {
	symbol     string
	connection []*websocket.Conn
}

var clientSubscriptionsData = []CSD{}

func main() {
	redis.InitRedis()
}
