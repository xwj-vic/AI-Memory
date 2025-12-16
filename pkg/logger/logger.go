package logger

import (
	"ai-memory/pkg/config"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var Log *slog.Logger
var rotatingWriter *RotatingWriter

// RotatingWriter 实现按天轮转的日志写入器
type RotatingWriter struct {
	logDir      string
	currentFile *os.File
	currentDate string
	mutex       sync.Mutex
}

// NewRotatingWriter 创建轮转写入器
func NewRotatingWriter(logDir string) (*RotatingWriter, error) {
	// 创建日志目录
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	rw := &RotatingWriter{
		logDir: logDir,
	}

	// 初始化当前日志文件
	if err := rw.rotate(time.Now().Format("2006-01-02")); err != nil {
		return nil, err
	}

	return rw, nil
}

// Write 实现 io.Writer 接口，自动处理日志轮转
func (rw *RotatingWriter) Write(p []byte) (n int, err error) {
	rw.mutex.Lock()
	defer rw.mutex.Unlock()

	today := time.Now().Format("2006-01-02")

	// 检查是否需要轮转
	if today != rw.currentDate {
		if err := rw.rotate(today); err != nil {
			// 轮转失败，继续使用当前文件
			fmt.Fprintf(os.Stderr, "Failed to rotate log file: %v\n", err)
		}
	}

	// 写入当前文件
	if rw.currentFile != nil {
		return rw.currentFile.Write(p)
	}

	return 0, fmt.Errorf("no log file available")
}

// rotate 切换到新的日志文件
func (rw *RotatingWriter) rotate(date string) error {
	// 关闭旧文件
	if rw.currentFile != nil {
		rw.currentFile.Close()
	}

	// 打开新文件
	logPath := filepath.Join(rw.logDir, date+".log")
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", logPath, err)
	}

	rw.currentFile = file
	rw.currentDate = date

	return nil
}

// Close 关闭日志文件
func (rw *RotatingWriter) Close() error {
	rw.mutex.Lock()
	defer rw.mutex.Unlock()

	if rw.currentFile != nil {
		return rw.currentFile.Close()
	}
	return nil
}

// Init 初始化日志系统（显式调用，替代原来的init）
func Init(cfg *config.Config) error {
	// 创建轮转写入器
	var err error
	rotatingWriter, err = NewRotatingWriter(cfg.LogDir)
	if err != nil {
		// 降级：如果无法创建轮转写入器，退回到stdout
		fmt.Fprintf(os.Stderr, "Failed to initialize rotating logger: %v\n", err)
		rotatingWriter = nil
	}

	// 组合输出：stdout + 轮转文件
	var writer io.Writer = os.Stdout
	if rotatingWriter != nil {
		writer = io.MultiWriter(os.Stdout, rotatingWriter)
	}

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// 自定义时间格式
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format(time.DateTime))
			}
			return a
		},
	}

	// 使用 TextHandler（生产环境可改为 JSONHandler）
	handler := slog.NewTextHandler(writer, opts)
	Log = slog.New(handler)

	// 记录初始化信息
	Log.Info("Logger initialized", slog.String("log_dir", cfg.LogDir))

	return nil
}

// Shutdown 优雅关闭日志系统
func Shutdown() {
	if rotatingWriter != nil {
		rotatingWriter.Close()
	}
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
