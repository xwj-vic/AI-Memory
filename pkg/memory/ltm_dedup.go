package memory

import (
	"ai-memory/pkg/logger"
	"ai-memory/pkg/types"
	"context"
	"math"
)

// DeduplicateLTM 定期扫描并去重LTM中的相似记忆
func (m *Manager) DeduplicateLTM(ctx context.Context) error {
	batchSize := 1000
	offset := 0
	processed := 0
	merged := 0

	for {
		// 1. 分批获取LTM记录
		records, err := m.vectorStore.List(ctx, map[string]interface{}{}, batchSize, offset)
		if err != nil || len(records) == 0 {
			break
		}

		// 2. 对当前批次构建相似度矩阵
		for i := 0; i < len(records); i++ {
			for j := i + 1; j < len(records); j++ {
				rec1, rec2 := records[i], records[j]

				// 只处理同一用户的记录
				user1, ok1 := rec1.Metadata["user_id"].(string)
				user2, ok2 := rec2.Metadata["user_id"].(string)
				if !ok1 || !ok2 || user1 != user2 {
					continue
				}

				// 计算余弦相似度
				similarity := cosineSimilarity(rec1.Embedding, rec2.Embedding)

				if similarity > 0.95 {
					// 3. 调用智能合并策略
					strategy, mergedContent, err := m.judge.DecideMergeStrategy(ctx, rec1.Content, rec2.Content)
					if err != nil {
						logger.Error("合并策略判定失败", err)
						continue
					}

					if err := m.executeMergeStrategy(ctx, rec1, rec2, strategy, mergedContent); err != nil {
						logger.Error("执行合并策略失败", err)
					} else {
						merged++
					}

					processed++
				}
			}
		}

		offset += batchSize
	}

	logger.System("LTM去重完成", "scanned", offset, "processed", processed, "merged", merged)
	return nil
}

// executeMergeStrategy 执行合并策略
func (m *Manager) executeMergeStrategy(
	ctx context.Context,
	rec1, rec2 types.Record,
	strategy, merged string,
) error {
	count1, _ := rec1.Metadata["access_count"].(int)
	count2, _ := rec2.Metadata["access_count"].(int)

	switch strategy {
	case "keep_newer":
		// 保留时间更新的记录
		if rec1.Timestamp.After(rec2.Timestamp) {
			m.vectorStore.Delete(ctx, []string{rec2.ID})
		} else {
			m.vectorStore.Delete(ctx, []string{rec1.ID})
		}

	case "keep_higher_access", "update_existing":
		// 保留访问次数更多的记录
		if count1 >= count2 {
			rec1.Metadata["access_count"] = count1 + count2
			rec1.Metadata["decay_score"] = 1.0
			m.vectorStore.Update(ctx, rec1)
			m.vectorStore.Delete(ctx, []string{rec2.ID})
		} else {
			rec2.Metadata["access_count"] = count1 + count2
			rec2.Metadata["decay_score"] = 1.0
			m.vectorStore.Update(ctx, rec2)
			m.vectorStore.Delete(ctx, []string{rec1.ID})
		}

	case "merge":
		// 合并为新记录，删除旧记录
		newVector, err := m.embedder.EmbedQuery(ctx, merged)
		if err != nil {
			return err
		}

		rec1.Content = merged
		rec1.Embedding = newVector
		rec1.Metadata["access_count"] = count1 + count2
		rec1.Metadata["decay_score"] = 1.0
		m.vectorStore.Update(ctx, rec1)
		m.vectorStore.Delete(ctx, []string{rec2.ID})

	case "keep_both":
		// 不做任何操作
		return nil
	}

	return nil
}

// cosineSimilarity 计算余弦相似度
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
