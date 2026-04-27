// Package main shows a minimal go-sdk bootstrap example.
package main

import (
	"context"
	"log"
	"time"

	"github.com/agent-assembly/go-sdk/assembly"
)

func main() {
	runtime := assembly.NewAssembly(
		assembly.WithGatewayURL("https://your-gateway.com"),
		assembly.WithAPIKey("xxx"),
		assembly.WithFailClosed(false),
		assembly.WithTimeout(500*time.Millisecond),
	)

	if err := runtime.Init(context.Background()); err != nil {
		log.Fatalf("init assembly runtime: %v", err)
	}
	defer func() {
		if err := runtime.Close(); err != nil {
			log.Printf("close assembly runtime: %v", err)
		}
	}()
}
