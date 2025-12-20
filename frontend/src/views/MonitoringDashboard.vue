<template>
  <div class="monitoring-dashboard">
    <el-page-header :icon="null">
      <template #content>
        <div class="page-header-content">
          <span class="header-title">{{ $t('monitoring.title') }}</span>
        </div>
      </template>
      <template #extra>
        <el-space :size="12">
          <el-select 
            v-model="timeRange" 
            @change="onTimeRangeChange" 
            :placeholder="$t('monitoring.timeRange')"
            style="width: 140px;"
          >
            <el-option :label="$t('monitoring.lastHour')" value="1h" />
            <el-option :label="$t('monitoring.last24Hours')" value="24h" />
            <el-option :label="$t('monitoring.last7Days')" value="7d" />
            <el-option :label="$t('monitoring.last30Days')" value="30d" />
          </el-select>
          
          <el-button @click="refreshMetrics" :loading="loading">
            <template #icon>
              <el-icon><Refresh /></el-icon>
            </template>
            {{ $t('common.refresh') }}
          </el-button>
          
          <el-button @click="exportData">
            <template #icon>
              <el-icon><Download /></el-icon>
            </template>
            {{ $t('common.exportCSV') }}
          </el-button>
        </el-space>
      </template>
    </el-page-header>

    <!-- 告警面板 -->
    <el-card v-if="recentAlerts.length > 0" class="alerts-card" shadow="hover">
      <template #header>
        <div class="card-header">
          <el-icon><Warning /></el-icon>
          <span>{{ $t('monitoring.recentAlerts') }}</span>
        </div>
      </template>
      <el-timeline>
        <el-timeline-item 
          v-for="alert in recentAlerts" 
          :key="alert.id"
          :type="alert.level === 'ERROR' ? 'danger' : alert.level === 'WARNING' ? 'warning' : 'info'"
          :timestamp="formatTime(alert.timestamp)"
          placement="top"
        >
          <el-tag :type="alert.level === 'ERROR' ? 'danger' : alert.level === 'WARNING' ? 'warning' : 'info'" size="small">
            {{ alert.level }}
          </el-tag>
          <p style="margin: 8px 0;">{{ alert.message }}</p>
          <el-descriptions v-if="alert.metadata" :column="2" size="small" border>
            <el-descriptions-item v-for="(value, key) in alert.metadata" :key="key" :label="key">
              {{ value }}
            </el-descriptions-item>
          </el-descriptions>
        </el-timeline-item>
      </el-timeline>
    </el-card>

    <el-row :gutter="20" class="metrics-row">
      <el-col :xs="12" :sm="12" :md="6">
        <el-card shadow="hover" class="stat-card">
          <el-statistic :title="$t('monitoring.totalPromotions')" :value="metrics.total_promotions || 0">
            <template #prefix>
              <el-icon color="#409EFF"><TrendCharts /></el-icon>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
      
      <el-col :xs="12" :sm="12" :md="6">
        <el-card shadow="hover" class="stat-card">
          <el-statistic :title="$t('monitoring.successRate')" :value="(metrics.promotion_success_rate || 0).toFixed(1)" suffix="%">
            <template #prefix>
              <el-icon color="#67C23A"><CircleCheckFilled /></el-icon>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
      
      <el-col :xs="12" :sm="12" :md="6">
        <el-card shadow="hover" class="stat-card">
          <el-statistic :title="$t('monitoring.queueLength')" :value="metrics.current_queue_length || 0">
            <template #prefix>
              <el-icon color="#E6A23C"><List /></el-icon>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
      
      <el-col :xs="12" :sm="12" :md="6">
        <el-card shadow="hover" class="stat-card">
          <el-statistic :title="$t('monitoring.cacheHitRate')" :value="(metrics.cache_hit_rate || 0).toFixed(1)" suffix="%">
            <template #prefix>
              <el-icon color="#F56C6C"><Coin /></el-icon>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
    </el-row>

    <!-- 图表区域 -->
    <div class="charts-grid">
      <!-- 晋升趋势图 -->
      <div class="chart-card">
        <h3>📈 {{ $t('monitoring.promotionTrend') }}</h3>
        <canvas ref="promotionChart"></canvas>
      </div>

      <!-- 队列长度曲线 -->
      <div class="chart-card">
        <h3>📊 {{ $t('monitoring.queueLengthChange') }}</h3>
        <canvas ref="queueChart"></canvas>
      </div>

      <!-- 分类分布饼图 -->
      <div class="chart-card">
        <h3>🥧 {{ $t('monitoring.categoryDistribution') }}</h3>
        <canvas ref="categoryChart"></canvas>
      </div>

      <!-- 信心等级分布 -->
      <div class="chart-card">
        <h3>🎯 {{ $t('monitoring.confidenceDistribution') }}</h3>
        <div class="confidence-bars">
          <div class="conf-bar">
            <div class="conf-label">{{ $t('monitoring.highConfidence') }}</div>
            <div class="conf-progress high">
              <div class="conf-fill" :style="{width: confidencePercent('high') + '%'}"></div>
            </div>
            <div class="conf-value">{{ metrics.high_confidence_count || 0 }}</div>
          </div>
          <div class="conf-bar">
            <div class="conf-label">{{ $t('monitoring.mediumConfidence') }}</div>
            <div class="conf-progress medium">
              <div class="conf-fill" :style="{width: confidencePercent('medium') + '%'}"></div>
            </div>
            <div class="conf-value">{{ metrics.medium_confidence_count || 0 }}</div>
          </div>
          <div class="conf-bar">
            <div class="conf-label">{{ $t('monitoring.lowConfidence') }}</div>
            <div class="conf-progress low">
              <div class="conf-fill" :style="{width: confidencePercent('low') + '%'}"></div>
            </div>
            <div class="conf-value">{{ metrics.low_confidence_count || 0 }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- 详细统计表格 -->
    <div class="details-section">
      <h3>📋 {{ $t('monitoring.detailedStats') }}</h3>
      <table class="metrics-table">
        <tr>
          <td>{{ $t('monitoring.totalPromotions') }}</td>
          <td class="value">{{ metrics.total_promotions || 0 }}</td>
          <td>{{ $t('monitoring.totalRejections') }}</td>
          <td class="value">{{ metrics.total_rejections || 0 }}</td>
        </tr>
        <tr>
          <td>{{ $t('monitoring.totalForgotten') }}</td>
          <td class="value">{{ metrics.total_forgotten || 0 }}</td>
          <td>{{ $t('monitoring.currentQueue') }}</td>
          <td class="value">{{ metrics.current_queue_length || 0 }}</td>
        </tr>
        <tr>
          <td>{{ $t('monitoring.cacheHits') }}</td>
          <td class="value">{{ metrics.cache_hits || 0 }}</td>
          <td>{{ $t('monitoring.cacheMisses') }}</td>
          <td class="value">{{ metrics.cache_misses || 0 }}</td>
        </tr>
      </table>
    </div>
  </div>
</template>

<script>
import { Chart, registerables } from 'chart.js'
Chart.register(...registerables)

export default {
  name: 'MonitoringDashboard',
  data() {
    return {
      metrics: {},
      charts: {},
      refreshInterval: null,
      recentAlerts: [],
      timeRange: '24h',
      loading: false
    }
  },
  computed: {
    totalConfidence() {
      return (this.metrics.high_confidence_count || 0) + 
             (this.metrics.medium_confidence_count || 0) + 
             (this.metrics.low_confidence_count || 0)
    }
  },
  watch: {
    '$i18n.locale'() {
      this.renderCharts()
    }
  },
  mounted() {
    this.loadMetrics()
    this.loadAlerts()
    this.refreshInterval = setInterval(() => {
      this.loadMetrics()
      this.loadAlerts()
    }, 10000) // 每10秒刷新
  },
  beforeUnmount() {
    if (this.refreshInterval) {
      clearInterval(this.refreshInterval)
    }
    Object.values(this.charts).forEach(chart => chart.destroy())
  },
  methods: {
    async loadMetrics() {
      try {
        const res = await fetch('/api/dashboard/metrics')
        this.metrics = await res.json()
        this.renderCharts()
      } catch (error) {
        console.error('加载监控数据失败:', error)
      }
    },
    async loadAlerts() {
      try {
        const res = await fetch('/api/alerts?limit=5')
        const data = await res.json()
        this.recentAlerts = data.alerts || []
      } catch (error) {
        console.error('加载告警数据失败:', error)
      }
    },
    async refreshMetrics() {
      this.loading = true
      await Promise.all([this.loadMetrics(), this.loadAlerts()])
      this.loading = false
    },
    onTimeRangeChange() {
      // TODO: 根据时间范围加载数据（需要后端支持）
      console.log('Time range changed to:', this.timeRange)
      this.refreshMetrics()
    },
    exportData() {
      try {
        // 生成CSV格式数据
        const csvData = this.generateCSV()
        // 添加 BOM (\uFEFF) 以解决 Excel 打开 UTF-8 CSV 中文乱码问题
        const blob = new Blob(['\uFEFF' + csvData], { type: 'text/csv;charset=utf-8;' })
        const url = URL.createObjectURL(blob)
        
        const link = document.createElement('a')
        link.href = url
        // 使用简单文件名，避免特殊字符问题
        link.download = `metrics_${new Date().getFullYear()}${String(new Date().getMonth()+1).padStart(2,'0')}${String(new Date().getDate()).padStart(2,'0')}.csv`
        link.style.display = 'none'
        document.body.appendChild(link)
        
        link.click()
        
        // 延迟清理，确保浏览器有足够时间处理下载
        setTimeout(() => {
          document.body.removeChild(link)
          URL.revokeObjectURL(url)
        }, 100)
        
        this.$message.success(this.$t('common.success'))
      } catch (e) {
        console.error('Export failed:', e)
        this.$message.error(this.$t('common.error'))
      }
    },
    generateCSV() {
      const headers = `${this.$t('monitoring.label')},${this.$t('monitoring.value')},${this.$t('monitoring.time')}`
      const rows = [
        `${this.$t('monitoring.totalPromotions')},${this.metrics.total_promotions || 0},${new Date().toISOString()}`,
        `${this.$t('monitoring.totalRejections')},${this.metrics.total_rejections || 0},${new Date().toISOString()}`,
        `${this.$t('monitoring.totalForgotten')},${this.metrics.total_forgotten || 0},${new Date().toISOString()}`,
        `${this.$t('monitoring.currentQueue')},${this.metrics.current_queue_length || 0},${new Date().toISOString()}`,
        `${this.$t('monitoring.successRate')}(%),${(this.metrics.promotion_success_rate || 0).toFixed(2)},${new Date().toISOString()}`,
        `${this.$t('monitoring.cacheHitRate')}(%),${(this.metrics.cache_hit_rate || 0).toFixed(2)},${new Date().toISOString()}`
      ]
      return [headers, ...rows].join('\n')
    },
    alertLevelClass(level) {
      return {
        'ERROR': 'alert-error',
        'WARNING': 'alert-warning',
        'INFO': 'alert-info'
      }[level] || 'alert-info'
    },
    formatTime(timestamp) {
      if (!timestamp) return ''
      const date = new Date(timestamp)
      const now = new Date()
      const diff = Math.floor((now - date) / 1000)
      
      if (diff < 60) return `${diff}秒前`
      if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`
      if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`
      return date.toLocaleString('zh-CN')
    },
    confidencePercent(level) {
      const total = this.totalConfidence
      if (total === 0) return 0
      
      const counts = {
        high: Number(this.metrics.high_confidence_count) || 0,
        medium: Number(this.metrics.medium_confidence_count) || 0,
        low: Number(this.metrics.low_confidence_count) || 0
      }
      
      return (counts[level] / total * 100).toFixed(1)
    },
    renderCharts() {
      this.renderPromotionChart()
      this.renderQueueChart()
      this.renderCategoryChart()
    },
    renderPromotionChart() {
      const ctx = this.$refs.promotionChart?.getContext('2d')
      if (!ctx) return

      if (this.charts.promotion) {
        this.charts.promotion.destroy()
      }

      const trend = this.metrics.promotion_trend || []
      
      this.charts.promotion = new Chart(ctx, {
        type: 'line',
        data: {
          labels: trend.map(p => new Date(p.timestamp).toLocaleTimeString('zh-CN', {hour: '2-digit', minute: '2-digit'})),
          datasets: [{
            label: this.$t('monitoring.promotionCount'),
            data: trend.map(p => p.value),
            borderColor: '#10b981',
            backgroundColor: 'rgba(16, 185, 129, 0.1)',
            tension: 0.4,
            fill: true
          }]
        },
        options: {
          responsive: true,
          maintainAspectRatio: true,
          plugins: {
            legend: { display: false }
          },
          scales: {
            y: { beginAtZero: true }
          }
        }
      })
    },
    renderQueueChart() {
      const ctx = this.$refs.queueChart?.getContext('2d')
      if (!ctx) return

      if (this.charts.queue) {
        this.charts.queue.destroy()
      }

      const trend = this.metrics.queue_length_trend || []
      
      this.charts.queue = new Chart(ctx, {
        type: 'line',
        data: {
          labels: trend.map(p => new Date(p.timestamp).toLocaleTimeString('zh-CN', {hour: '2-digit', minute: '2-digit'})),
          datasets: [{
            label: this.$t('monitoring.queueLen'),
            data: trend.map(p => p.value),
            borderColor: '#3b82f6',
            backgroundColor: 'rgba(59, 130, 246, 0.1)',
            tension: 0.4,
            fill: true
          }]
        },
        options: {
          responsive: true,
          maintainAspectRatio: true,
          plugins: {
            legend: { display: false }
          },
          scales: {
            y: { beginAtZero: true }
          }
        }
      })
    },
    renderCategoryChart() {
      const ctx = this.$refs.categoryChart?.getContext('2d')
      if (!ctx) return

      if (this.charts.category) {
        this.charts.category.destroy()
      }

      const distribution = this.metrics.category_distribution || []
      
      const categoryLabels = {
        'fact': this.$t('staging.categories.fact'),
        'preference': this.$t('staging.categories.preference'),
        'goal': this.$t('staging.categories.goal'),
        'noise': this.$t('staging.categories.noise')
      }
      
      this.charts.category = new Chart(ctx, {
        type: 'doughnut',
        data: {
          labels: distribution.map(d => categoryLabels[d.category] || d.category),
          datasets: [{
            data: distribution.map(d => d.count),
            backgroundColor: [
              '#3b82f6', // 事实-蓝
              '#ec4899', // 偏好-粉
              '#10b981', // 目标-绿
              '#ef4444'  // 噪音-红
            ]
          }]
        },
        options: {
          responsive: true,
          maintainAspectRatio: true,
          plugins: {
            legend: {
              position: 'bottom'
            }
          }
        }
      })
    }
  }
}
</script>

<style scoped>
.monitoring-dashboard {
  padding: 2rem;
  width: 100%;
  margin: 0;
}

h1 {
  margin-bottom: 2rem;
  color: #1f2937;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1.5rem;
  margin-bottom: 2rem;
}

.stat-card {
  background: white;
  border-radius: 12px;
  padding: 1.5rem;
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
  display: flex;
  align-items: center;
  gap: 1rem;
  transition: transform 0.2s;
}

.stat-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 4px 12px rgba(0,0,0,0.15);
}

.stat-icon {
  font-size: 2.5rem;
}

.stat-value {
  font-size: 2rem;
  font-weight: bold;
  color: #1f2937;
}

.stat-label {
  color: #6b7280;
  font-size: 0.875rem;
  margin-top: 0.25rem;
}

.charts-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
  gap: 1.5rem;
  margin-bottom: 2rem;
}

.chart-card {
  background: white;
  border-radius: 12px;
  padding: 1.5rem;
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
}

.chart-card h3 {
  margin: 0 0 1rem 0;
  color: #374151;
  font-size: 1.125rem;
}

canvas {
  max-height: 250px;
}

.confidence-bars {
  padding: 1rem 0;
}

.conf-bar {
  display: grid;
  grid-template-columns: 80px 1fr 60px;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1rem;
}

.conf-label {
  font-weight: 500;
  color: #4b5563;
}

.conf-progress {
  height: 24px;
  background: #f3f4f6;
  border-radius: 12px;
  overflow: hidden;
}

.conf-fill {
  height: 100%;
  transition: width 0.3s ease;
  border-radius: 12px;
}

.conf-progress.high .conf-fill {
  background: linear-gradient(90deg, #10b981, #059669);
}

.conf-progress.medium .conf-fill {
  background: linear-gradient(90deg, #f59e0b, #d97706);
}

.conf-progress.low .conf-fill {
  background: linear-gradient(90deg, #ef4444, #dc2626);
}

.conf-value {
  text-align: right;
  font-weight: bold;
  color: #1f2937;
}

.details-section {
  background: white;
  border-radius: 12px;
  padding: 1.5rem;
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
}

.details-section h3 {
  margin: 0 0 1rem 0;
  color: #374151;
}

.metrics-table {
  width: 100%;
  border-collapse: collapse;
}

.metrics-table td {
  padding: 0.75rem 1rem;
  border-bottom: 1px solid #e5e7eb;
}

.metrics-table td.value {
  font-weight: bold;
  color: #3b82f6;
  text-align: right;
}

.metrics-table tr:last-child td {
  border-bottom: none;
}
</style>

/* 工具栏样式 */
.dashboard-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2rem;
}

.dashboard-header h1 {
  margin: 0;
}

.toolbar {
  display: flex;
  gap: 1rem;
  align-items: center;
}

.time-selector {
  padding: 0.5rem 1rem;
  border: 1px solid #d1d5db;
  border-radius: 8px;
  background: white;
  font-size: 0.95rem;
  cursor: pointer;
  transition: all 0.2s;
}

.time-selector:hover {
  border-color: #3b82f6;
}

.btn-icon {
  padding: 0.5rem 1rem;
  border: 1px solid #d1d5db;
  border-radius: 8px;
  background: white;
  cursor: pointer;
  font-size: 0.95rem;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.btn-icon:hover:not(:disabled) {
  background: #f3f4f6;
  border-color: #3b82f6;
}

.btn-icon:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* 告警面板样式 */
.alerts-panel {
  background: white;
  border-radius: 12px;
  padding: 1.5rem;
  margin-bottom: 2rem;
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
}

.alerts-panel h3 {
  margin: 0 0 1rem 0;
  color: #374151;
  font-size: 1.125rem;
}

.alerts-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.alert-item {
  padding: 1rem;
  border-radius: 8px;
  border-left: 4px solid;
  transition: all 0.2s;
}

.alert-item:hover {
  transform: translateX(4px);
}

.alert-error {
  background: #fef2f2;
  border-left-color: #ef4444;
}

.alert-warning {
  background: #fffbeb;
  border-left-color: #f59e0b;
}

.alert-info {
  background: #eff6ff;
  border-left-color: #3b82f6;
}

.alert-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.5rem;
}

.alert-level-badge {
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  background: rgba(0,0,0,0.1);
}

.alert-time {
  font-size: 0.875rem;
  color: #6b7280;
}

.alert-message {
  font-size: 0.95rem;
  color: #1f2937;
  margin-bottom: 0.5rem;
}

.alert-metadata {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.metadata-item {
  font-size: 0.8rem;
  color: #6b7280;
  background: rgba(0,0,0,0.05);
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
}
