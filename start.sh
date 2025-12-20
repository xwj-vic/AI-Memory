#!/bin/bash

set -e

PORT=8080

# 解析命令行参数
MODE="all"
if [ "$1" == "--frontend-only" ] || [ "$1" == "-f" ]; then
    MODE="frontend"
fi

echo "=== AI Memory System Startup ==="
echo ""

# 根据模式显示信息
if [ "$MODE" == "frontend" ]; then
    echo "模式: 仅启动前端开发服务器"
else
    echo "模式: 完整启动 (前端 + 后端)"
fi

echo ""

# 前端处理
if [ "$MODE" == "frontend" ]; then
    echo "=== Starting Frontend Dev Server ==="
    cd frontend
    npm run dev
    exit 0
fi

# 完整模式：检查并清理端口占用
echo "🔍 检查端口 $PORT..."
if lsof -Pi :$PORT -sTCP:LISTEN -t >/dev/null 2>&1; then
    echo "⚠️  端口 $PORT 已被占用，正在清理..."
    PIDS=$(lsof -ti:$PORT)
    if [ -n "$PIDS" ]; then
        echo "   杀掉进程: $PIDS"
        kill -9 $PIDS 2>/dev/null || true
        sleep 2
        echo "✅ 端口已释放"
    fi
else
    echo "✅ 端口 $PORT 可用"
fi

echo ""
echo "=== Building Frontend ==="
cd frontend
npm run build
cd ..

echo ""
echo "=== Starting Backend ==="
echo "Serving frontend from ./frontend/dist"
echo "Admin Dashboard available at http://localhost:$PORT"
echo ""
echo "提示: 若要仅启动前端开发服务器，使用: ./start.sh --frontend-only"
echo ""

# 启动后端
echo "Compiling Backend..."
go build -o ai-memory
if [ $? -ne 0 ]; then
    echo "❌ Compilation failed, aborting startup."
    exit 1
fi
echo "✅ Compilation successful."

./ai-memory

# 如果想后台运行，使用：
# nohup ./ai-memory > ai-memory.log 2>&1 &
# echo "✅ 后端已在后台启动 (PID: $!)"
# echo "📋 查看日志: tail -f ai-memory.log"
