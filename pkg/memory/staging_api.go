package memory

import (
	"ai-memory/pkg/types"
	"context"
	"fmt"
	"time"
)

// GetStagingEntries 获取所有暂存区条目（Admin用）
func (m *Manager) GetStagingEntries(ctx context.Context, userID string) ([]*types.StagingEntry, error) {
	if userID != "" {
		return m.stagingStore.GetAllByUser(ctx, userID)
	}
	// 获取所有待处理的（用于管理界面）
	return m.stagingStore.GetPendingEntries(ctx, 1, 0) // 至少1次，不限时间
}

// ConfirmStagingEntry 用户确认暂存区记忆并晋升到LTM
func (m *Manager) ConfirmStagingEntry(ctx context.Context, entryID string) error {
	// 获取条目
	entries, err := m.stagingStore.GetPendingEntries(ctx, 1, 0)
	if err != nil {
		return fmt.Errorf("获取暂存区条目失败: %w", err)
	}

	var targetEntry *types.StagingEntry
	for _, entry := range entries {
		if entry.ID == entryID {
			targetEntry = entry
			break
		}
	}

	if targetEntry == nil {
		return fmt.Errorf("条目不存在: %s", entryID)
	}

	// 标记为已确认
	targetEntry.Status = types.StagingConfirmed
	targetEntry.ConfirmedBy = "user"

	// 晋升到LTM
	return m.promoteSingleEntry(ctx, targetEntry, "user")
}

// RejectStagingEntry 用户拒绝暂存区记忆
func (m *Manager) RejectStagingEntry(ctx context.Context, entryID string) error {
	return m.stagingStore.Delete(ctx, entryID)
}

// GetStagingStats 获取暂存区统计信息
func (m *Manager) GetStagingStats(ctx context.Context) (map[string]interface{}, error) {
	allEntries, err := m.stagingStore.GetPendingEntries(ctx, 1, 0)
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_pending":      len(allEntries),
		"high_confidence":    0,
		"medium_confidence":  0,
		"low_confidence":     0,
		"awaiting_promotion": 0,
	}

	for _, entry := range allEntries {
		if entry.ConfidenceScore >= m.cfg.StagingConfidenceHigh {
			if val, ok := stats["high_confidence"].(int); ok {
				stats["high_confidence"] = val + 1
			}
		} else if entry.ConfidenceScore >= m.cfg.StagingConfidenceLow {
			if val, ok := stats["medium_confidence"].(int); ok {
				stats["medium_confidence"] = val + 1
			}
		} else {
			if val, ok := stats["low_confidence"].(int); ok {
				stats["low_confidence"] = val + 1
			}
		}

		// 检查是否达到晋升条件
		if entry.FirstSeenAt.IsZero() {
			continue
		}
		hoursSinceFirst := time.Since(entry.FirstSeenAt).Hours()
		if entry.OccurrenceCount >= m.cfg.StagingMinOccurrences &&
			int(hoursSinceFirst) >= m.cfg.StagingMinWaitHours {
			if val, ok := stats["awaiting_promotion"].(int); ok {
				stats["awaiting_promotion"] = val + 1
			}
		}
	}

	return stats, nil
}
