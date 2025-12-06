package main

import (
	"async-service/internal/api"
	"log"
)

func main() {
	log.Println("Application start up")
	api.StartServer()
	log.Println("Application terminated")
}
