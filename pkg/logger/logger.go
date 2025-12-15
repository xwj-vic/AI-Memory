package logger

import (
	"context"
	"log/slog"
	"os"
	"time"
)

var Log *slog.Logger

func init() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// 可以在这里自定义时间格式等
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format(time.DateTime))
			}
			return a
		},
	}
	// 使用 JSON Handler 以便机器解析，或者 Text Handler 用于开发调试
	// 为了美观，这里暂时用 TextHandler，生产环境建议 JSON
	handler := slog.NewTextHandler(os.Stdout, opts)
	Log = slog.New(handler)
}

// System 记录系统级关键事件
func System(msg string, args ...any) {
	Log.Info("[SYSTEM] "+msg, args...)
}

// LLM 记录 LLM 调用详情
func LLM(ctx context.Context, model, promptType string, duration time.Duration, err error) {
	attrs := []any{
		slog.String("module", "llm"),
		slog.String("model", model),
		slog.String("type", promptType),
		slog.Duration("latency", duration),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		Log.Error("LLM Call Failed", attrs...)
	} else {
		Log.Info("LLM Call Success", attrs...)
	}
}

// MemoryPromotion 记录记忆晋升
func MemoryPromotion(category, id string, score float64, reason string) {
	Log.Info("🧠 Memory Promotion",
		slog.String("category", category),
		slog.String("id", id),
		slog.Float64("score", score),
		slog.String("reason", reason),
	)
}

// MemoryCheck 记录记忆检查/判定
func MemoryCheck(action string, count int, details string) {
	Log.Info("🔍 Memory Check",
		slog.String("action", action),
		slog.Int("count", count),
		slog.String("details", details),
	)
}

// Error 简单包装
func Error(msg string, err error) {
	Log.Error(msg, slog.String("error", err.Error()))
}

// Info 简单包装
func Info(msg string, args ...any) {
	Log.Info(msg, args...)
}
