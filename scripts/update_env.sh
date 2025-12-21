#!/bin/bash

# update_env.sh - 自动更新 .env 文件添加新配置
# 用法: ./scripts/update_env.sh

set -e

ENV_FILE=".env"
BACKUP_FILE=".env.backup.$(date +%Y%m%d_%H%M%S)"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "========================================="
echo "  告警系统配置自动更新脚本"
echo "========================================="
echo

# 检查 .env 文件是否存在
if [ ! -f "$ENV_FILE" ]; then
    echo -e "${RED}错误: .env 文件不存在${NC}"
    echo "请先从 .env.example 复制创建 .env 文件："
    echo "  cp .env.example .env"
    exit 1
fi

# 备份原文件
echo -e "${YELLOW}📦 备份原文件到: $BACKUP_FILE${NC}"
cp "$ENV_FILE" "$BACKUP_FILE"

# 检查是否已经有新配置
if grep -q "ALERT_CACHE_WINDOW_MINUTES" "$ENV_FILE"; then
    echo -e "${YELLOW}⚠️  检测到已存在智能缓存配置，跳过添加${NC}"
    echo
    echo "当前配置："
    grep "ALERT_CACHE" "$ENV_FILE" | grep -v "HIT_RATE_THRESHOLD" || true
    echo
    echo -e "${GREEN}✅ 无需更新${NC}"
    exit 0
fi

# 添加新配置
echo -e "${GREEN}📝 添加智能缓存检测配置...${NC}"

cat >> "$ENV_FILE" << 'EOF'

# ========== 智能缓存检测配置（2025-12-21 新增）==========
# 优化缓存告警逻辑，减少误报
ALERT_CACHE_WINDOW_MINUTES=5          # 统计窗口（分钟）
ALERT_CACHE_MIN_SAMPLES=500           # 最小样本数
ALERT_CACHE_WARN_THRESHOLD=30         # 警告阈值（百分比）
ALERT_CACHE_ERROR_THRESHOLD=15        # 错误阈值（百分比）
ALERT_CACHE_TREND_PERIODS=3           # 趋势检测周期数
EOF

echo
echo -e "${GREEN}✅ 配置更新完成！${NC}"
echo
echo "新增配置："
echo "----------------------------------------"
grep "ALERT_CACHE" "$ENV_FILE" | grep -v "HIT_RATE_THRESHOLD" || true
echo "----------------------------------------"
echo
echo -e "${YELLOW}📌 下一步：${NC}"
echo "  1. 检查配置是否正确"
echo "  2. 重启服务使配置生效："
echo "     ./start.sh"
echo
echo -e "${YELLOW}💾 原配置已备份至: $BACKUP_FILE${NC}"
echo "如需回滚，执行："
echo "  mv $BACKUP_FILE $ENV_FILE"
echo

exit 0
