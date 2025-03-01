package main 

import(
	"fmt"
)

func main() {
	fmt.Println("Client Start")
	serverAddress := "localhost:8080"
	clientStart(serverAddress)
}