package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/adityadeshlahre/probo-v1/shared/redis"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all origins (for now)
	},
}

type CSD struct {
	Symbol      string
	Subscribers []*websocket.Conn
}

var subscriptionsMap = []CSD{}

type OrderBookMessage struct {
	Event   string      `json:"event"`
	Message interface{} `json:"message"`
}

var ctx = context.Background()

func startRedisSubscription(symbol string) {
	go func() {
		pubsub := redis.GetRedisClient().Subscribe(ctx, symbol)
		defer pubsub.Close()

		for msg := range pubsub.Channel() {
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &data); err != nil {
				log.Println("Error parsing Redis message:", err)
				continue
			}

			sym, _ := data["symbol"].(string)
			SendOrderBookToSubscribers(sym, data)
		}
	}()
}

func SendOrderBookToSubscribers(symbol string, orderBook map[string]interface{}) {
	log.Println("Sending order book to subscribers for:", symbol)

	for _, sub := range subscriptionsMap {
		if sub.Symbol == symbol {
			for _, conn := range sub.Subscribers {
				msg := OrderBookMessage{
					Event:   "event_orderbook_update",
					Message: orderBook,
				}
				data, err := json.Marshal(msg)
				if err != nil {
					log.Println("Error marshalling JSON:", err)
					continue
				}

				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					log.Println("Error writing to client:", err)
				}
			}
			return
		}
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	defer conn.Close()

	log.Println(time.Now().Format(time.RFC3339), "New client connected")

	if err := conn.WriteMessage(websocket.TextMessage, []byte("connection successfull")); err != nil {
		log.Println("Write error:", err)
		return
	}

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			handleConnectionClosed(conn)
			break
		}

		handleIncomingMessages(conn, msgType, msg)
	}
}

type IncomingMessage struct {
	Type   string `json:"type"`
	Symbol string `json:"symbol"`
}

func handleIncomingMessages(conn *websocket.Conn, msgType int, msg []byte) {
	log.Println("Received message:", string(msg))

	var incoming IncomingMessage
	if err := json.Unmarshal(msg, &incoming); err != nil {
		log.Println("Error parsing JSON:", err)
		return
	}

	method := incoming.Type
	symbol := incoming.Symbol

	if method == "" {
		log.Println("Missing method")
		return
	}

	if symbol == "" {
		log.Println("Missing symbol")
		return
	}

	if method == "subscribe" {
		var subscription *CSD
		for i := range subscriptionsMap {
			if subscriptionsMap[i].Symbol == symbol {
				subscription = &subscriptionsMap[i]
				break
			}
		}

		if subscription == nil {
			newSub := CSD{
				Symbol:      symbol,
				Subscribers: []*websocket.Conn{},
			}
			subscriptionsMap = append(subscriptionsMap, newSub)
			subscription = &subscriptionsMap[len(subscriptionsMap)-1]

			// start listening to Redis for this symbol
			startRedisSubscription(symbol)
		}

		subscription.Subscribers = append(subscription.Subscribers, conn)
		conn.WriteMessage(websocket.TextMessage, []byte("Subscribed to "+symbol))

	} else if method == "unsubscribe" {
		for i := range subscriptionsMap {
			if subscriptionsMap[i].Symbol == symbol {
				newList := []*websocket.Conn{}
				for _, c := range subscriptionsMap[i].Subscribers {
					if c != conn {
						newList = append(newList, c)
					}
				}
				subscriptionsMap[i].Subscribers = newList

				break
			}
		}
	}
}

func handleConnectionClosed(conn *websocket.Conn) {
	fmt.Println("Client disconnected")
	// Remove connection from all subscriptions
	for i, csd := range subscriptionsMap {
		for j, c := range csd.Subscribers {
			if c == conn {
				// Remove connection
				subscriptionsMap[i].Subscribers = append(subscriptionsMap[i].Subscribers[:j], subscriptionsMap[i].Subscribers[j+1:]...)
				break
			}
		}
		// If no connections left for this symbol, remove the CSD entry
		// TODO: stop subscription if no subscribers
		// if len(subscriptions[i].Subscribers) == 0 {
		// 	stopRedisSubscription(symbol)
		// }
	}
}

func main() {
	redis.InitRedis()

	http.HandleFunc("/ws", wsHandler)

	fmt.Println("WebSocket server listening on :8081")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
