package main

import (
	"context"
	"fmt"
	"os"

	"ai-memory/pkg/config"
	"ai-memory/pkg/logger"
	"ai-memory/pkg/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Load config failed", err)
		os.Exit(1)
	}
	ctx := context.Background()

	// 初始化 Qdrant Store
	// client, err := qdrant.NewClient(...) // We use store package
	qs, err := store.NewQdrantStore(cfg)
	if err != nil {
		logger.Error("Qdrant connect failed", err)
		os.Exit(1)
	}

	coll := cfg.QdrantCollection
	logger.Info("Deleting collection", "collection", coll)

	err = qs.DeleteCollection(ctx, coll)
	if err != nil {
		logger.Info("Delete warning (might not exist)", "error", err)
	} else {
		fmt.Println("Collection deleted successfully.")
	}
}
