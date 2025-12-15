package main

import (
	"context"
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

	// 1. Check Redis
	redisStore := store.NewRedisStore(cfg)
	if err := redisStore.Ping(ctx); err == nil {
		// Count items?
		// pattern: memory:stm:*:*
		keys, _ := redisStore.ScanKeys(ctx, "memory:stm:*:*")
		items := 0
		for _, key := range keys {
			l, _ := redisStore.LRange(ctx, key, 0, -1)
			items += len(l)
		}
		logger.System("Redis (STM) Status", "items", items)
	} else {
		logger.Error("Redis (STM) Connection failed", err)
	}

	// 2. Check Qdrant
	qs, err := store.NewQdrantStore(cfg)
	if err != nil {
		logger.Error("Qdrant connect failed", err)
		os.Exit(1)
	}
	coll := cfg.QdrantCollection
	info, err := qs.GetCollectionInfo(context.Background(), coll)
	if err != nil {
		logger.Error("Qdrant (LTM) Failed to get info", err, "collection", coll)
	} else {
		logger.System("Qdrant (LTM) Collection Info",
			"collection", coll,
			"status", info.Status,
			"points_count", info.PointsCount)
	}
}
