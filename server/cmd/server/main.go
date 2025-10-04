package main

import (
	"github.com/adityadeshlahre/probo-v1/server/server"
)

const DefaultContextTimeout = 30

func main() {
	e := server.NewServer()
	e.Logger.Fatal(e.Start(":8080"))
}
