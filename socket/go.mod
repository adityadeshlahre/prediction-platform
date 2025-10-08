module github.com/adityadeshlahre/probo-v1/socket

go 1.25.1

require (
	github.com/adityadeshlahre/probo-v1/shared v0.0.0
	github.com/gorilla/websocket v1.5.3
	github.com/redis/go-redis/v9 v9.14.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/joho/godotenv v1.5.1 // indirect
)

replace github.com/adityadeshlahre/probo-v1/shared => ../shared
