#!/bin/bash

# apply_alert_rule_migration.sh - 应用告警规则配置表迁移
# 用法: ./scripts/apply_alert_rule_migration.sh

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "========================================="
echo "  告警规则配置表迁移"
echo "========================================="
echo

# 加载环境变量
if [ -f .env ]; then
    source <(grep -v '^#' .env | grep -v '^$' | sed 's/^/export /')
fi


# 数据库连接信息
DB_HOST=${DB_HOST:-localhost:3306}
DB_USER=${DB_USER:-root}
DB_PASS=${DB_PASS:-}
DB_NAME=${DB_NAME:-ai_memory}

echo -e "${YELLOW}📦 数据库连接信息:${NC}"
echo "  Host: $DB_HOST"
echo "  User: $DB_USER"
echo "  Database: $DB_NAME"
echo

# 执行迁移
echo -e "${GREEN}📝 应用迁移脚本...${NC}"

mysql -h "${DB_HOST%:*}" -P "${DB_HOST#*:}" -u "$DB_USER" ${DB_PASS:+-p"$DB_PASS"} "$DB_NAME" < migrations/002_alert_rule_configs.sql

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ 迁移成功完成！${NC}"
    echo
    echo "新增表: alert_rule_configs"
    echo "默认规则已插入: 4条"
    echo
    echo -e "${YELLOW}📌 下一步：${NC}"
    echo "  1. 重启服务使配置生效"
    echo "  2. 访问告警中心测试规则修改"
    echo "  3. 重启后验证配置是否保留"
else
    echo -e "${YELLOW}⚠️  迁移失败${NC}"
    exit 1
fi
