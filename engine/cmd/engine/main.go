package main

import server "github.com/adityadeshlahre/probo-v1/engine/handler"

const DefaultContextTimeout = 30

func main() {
	e := server.NewServer()
	e.Logger.Fatal(e.Start(":8082"))
}
