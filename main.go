package main

import (
	"myapp/server"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		server.RunWebServer()
	}()

	go func() {
		defer wg.Done()
		server.RunWebsocket()
	}()

	wg.Wait()
}
