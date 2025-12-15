package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"
)

var Log *slog.Logger

func init() {
	// 1. 打开日志文件
	logFile, err := os.OpenFile("ai_memory.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// 如果无法打开文件，仅回退到 Stdout，但打印错误
		os.Stderr.WriteString("Failed to open log file: " + err.Error() + "\n")
	}

	var writer io.Writer = os.Stdout
	if logFile != nil {
		writer = io.MultiWriter(os.Stdout, logFile)
	}

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
	handler := slog.NewTextHandler(writer, opts)
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
func Error(msg string, err error, args ...any) {
	// 将 error 加入 args
	if err != nil {
		args = append(args, slog.String("error", err.Error()))
	}
	Log.Error(msg, args...)
}

// Info 简单包装
func Info(msg string, args ...any) {
	Log.Info(msg, args...)
}
