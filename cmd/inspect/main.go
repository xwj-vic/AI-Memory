package main

import (
	"context"
	"fmt"
	"log"

	"ai-memory/pkg/config"
	"ai-memory/pkg/store"

	"github.com/qdrant/go-client/qdrant"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Load config failed: %v", err)
	}
	ctx := context.Background()

	// 1. Inspect Redis
	redisStore := store.NewRedisStore(cfg)
	if err := redisStore.Ping(ctx); err == nil {
		items, _ := redisStore.LRange(ctx, "memory:stm:chat_history", 0, -1)
		fmt.Printf("Redis (STM): %d items found.\n", len(items))
	} else {
		fmt.Printf("Redis (STM): Connection failed.\n")
	}

	// 2. Inspect Qdrant
	// We use raw client to get collection info
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: "localhost",
		Port: 6334,
	})
	if err != nil {
		log.Fatalf("Qdrant connect failed: %v", err)
	}

	coll := cfg.QdrantCollection
	info, err := client.GetCollectionInfo(ctx, coll)
	if err != nil {
		fmt.Printf("Qdrant (LTM): Failed to get info for '%s': %v\n", coll, err)
	} else {
		fmt.Printf("Qdrant (LTM) Collection '%s':\n", coll)
		fmt.Printf(" - Status: %v\n", info.Status)
		fmt.Printf(" - Points Count: %d\n", info.PointsCount)
	}
}
