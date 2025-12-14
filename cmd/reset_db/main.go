package main

import (
	"context"
	"fmt"
	"log"

	"ai-memory/pkg/config"

	"github.com/qdrant/go-client/qdrant"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Load config failed: %v", err)
	}
	ctx := context.Background()

	client, err := qdrant.NewClient(&qdrant.Config{
		Host: "localhost",
		Port: 6334,
	})
	if err != nil {
		log.Fatalf("Qdrant connect failed: %v", err)
	}

	coll := cfg.QdrantCollection
	fmt.Printf("Deleting collection '%s'...\n", coll)

	err = client.DeleteCollection(ctx, coll)
	if err != nil {
		// Log but continue - it might not exist
		fmt.Printf("Delete warning (might not exist): %v\n", err)
	} else {
		fmt.Println("Collection deleted successfully.")
	}
}
