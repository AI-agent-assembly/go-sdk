package main

import (
	"log"

	"github.com/agent-assembly/go-sdk/assembly"
)

func main() {
	err := assembly.InitAssembly(assembly.Config{
		Gateway:        "https://your-gateway.com",
		APIKey:         "xxx",
		SidecarAddress: "127.0.0.1:50051",
	})
	if err != nil {
		log.Fatalf("init assembly: %v", err)
	}
}
