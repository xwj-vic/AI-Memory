# 环境变量配置更新说明

## 需要更新的文件

### 1. `.env.example` (已更新 ✅)
已添加智能缓存检测的新配置项。

### 2. `.env` (需手动更新 ⚠️)
`.env` 文件被 `.gitignore` 保护，请手动添加以下配置：

```bash
# 在 .env 文件的告警配置部分添加以下内容

# 智能缓存检测配置（优化后，减少误报）
ALERT_CACHE_WINDOW_MINUTES=5          # 统计窗口（分钟），仅统计最近N分钟的数据
ALERT_CACHE_MIN_SAMPLES=500           # 最小样本数，低于此值不检测（避免冷启动和低流量误报）
ALERT_CACHE_WARN_THRESHOLD=30         # 警告阈值（百分比），低于此值触发WARNING
ALERT_CACHE_ERROR_THRESHOLD=15        # 错误阈值（百分比），低于此值触发ERROR
ALERT_CACHE_TREND_PERIODS=3           # 趋势检测周期数，检测命中率突降
```

## 快速更新命令

```bash
# 方法1: 手动编辑
vim .env
# 在告警配置部分添加上述5行配置

# 方法2: 追加到文件末尾（如果还没有这些配置）
cat >> .env << 'EOF'

# 智能缓存检测配置（优化后，减少误报）
ALERT_CACHE_WINDOW_MINUTES=5
ALERT_CACHE_MIN_SAMPLES=500
ALERT_CACHE_WARN_THRESHOLD=30
ALERT_CACHE_ERROR_THRESHOLD=15
ALERT_CACHE_TREND_PERIODS=3
EOF

# 方法3: 从.env.example复制
# 如果.env不存在或想重置
cp .env.example .env
# 然后填写你的实际配置（API密钥等）
```

## 配置说明

### 新配置项详解

- `ALERT_CACHE_WINDOW_MINUTES=5`  
  **统计窗口**: 只统计最近5分钟的数据，而不是全局累计。避免历史数据影响当前判断。

- `ALERT_CACHE_MIN_SAMPLES=500`  
  **最小样本数**: 至少需要500次缓存访问才进行检测。  
  **效果**: 避免冷启动和夜间低流量期间误报（旧值120太小）。

- `ALERT_CACHE_WARN_THRESHOLD=30`  
  **警告阈值**: 缓存命中率低于30%时触发WARNING级别告警。  
  **用途**: 提前预警，给运维时间排查。

- `ALERT_CACHE_ERROR_THRESHOLD=15`  
  **错误阈值**: 缓存命中率低于15%时触发ERROR级别告警。  
  **用途**: 严重问题，需要立即处理。

- `ALERT_CACHE_TREND_PERIODS=3`  
  **趋势检测周期**: 对比最近3个检测周期的平均值。  
  **效果**: 检测命中率突降（下降>20%），而不是单纯的低值。

### 旧配置项说明

- `ALERT_CACHE_HIT_RATE_THRESHOLD=20`  
  ⚠️ **已废弃**: 不再使用，但保留以避免配置错误。  
  新逻辑使用上述5个智能配置替代。

## 验证配置

```bash
# 检查配置是否生效
grep "ALERT_CACHE" .env

# 应该看到至少这5行
# ALERT_CACHE_WINDOW_MINUTES=5
# ALERT_CACHE_MIN_SAMPLES=500
# ALERT_CACHE_WARN_THRESHOLD=30
# ALERT_CACHE_ERROR_THRESHOLD=15
# ALERT_CACHE_TREND_PERIODS=3
```

## 重启服务使配置生效

```bash
# 重启服务
./start.sh

# 查看日志确认配置加载
tail -f log/system_*.log | grep "Alert engine started"
# 应该看到: Alert engine started with 4 rules
```

## 配置调优建议

根据你的实际业务场景，可能需要调整：

### 高流量场景（每分钟>1000次访问）
```bash
ALERT_CACHE_MIN_SAMPLES=1000     # 提高样本要求
ALERT_CACHE_WINDOW_MINUTES=3     # 缩短窗口，更快响应
```

### 低流量场景（每分钟<100次访问）
```bash
ALERT_CACHE_MIN_SAMPLES=300      # 降低样本要求
ALERT_CACHE_WINDOW_MINUTES=10    # 扩大窗口，避免数据不足
```

### 严格监控（对缓存极其敏感）
```bash
ALERT_CACHE_WARN_THRESHOLD=40    # 提高警告线
ALERT_CACHE_ERROR_THRESHOLD=20   # 提高错误线
```

### 宽松监控（减少告警噪音）
```bash
ALERT_CACHE_WARN_THRESHOLD=20    # 降低警告线
ALERT_CACHE_ERROR_THRESHOLD=10   # 降低错误线
```

## 注意事项

1. **不要删除旧配置**: `ALERT_CACHE_HIT_RATE_THRESHOLD` 虽然已废弃，但保留它可以避免配置缺失错误。

2. **重启后生效**: 配置修改后必须重启服务才能生效。

3. **观察一段时间**: 建议运行1-2天后，根据实际告警频率调整阈值。

4. **告警通知**: 如果启用了Webhook或Email通知，确保 `ALERT_NOTIFY_LEVELS` 包含你想接收的级别：
   ```bash
   ALERT_NOTIFY_LEVELS=ERROR,WARNING  # 接收错误和警告
   # 或
   ALERT_NOTIFY_LEVELS=ERROR          # 只接收错误
   ```
