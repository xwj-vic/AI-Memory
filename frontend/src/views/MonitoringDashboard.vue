<template>
  <div class="monitoring-dashboard">
    <div class="dashboard-header">
      <h1>📊 记忆系统监控中心</h1>
      
      <!-- 工具栏 -->
      <div class="toolbar">
        <select v-model="timeRange" @change="onTimeRangeChange" class="time-selector">
          <option value="1h">最近1小时</option>
          <option value="24h">最近24小时</option>
          <option value="7d">最近7天</option>
          <option value="30d">最近30天</option>
        </select>
        
        <button @click="refreshMetrics" class="btn btn-icon" :disabled="loading">
          <span v-if="loading">⏳</span>
          <span v-else>🔄</span> 刷新
        </button>
        
        <button @click="exportData" class="btn btn-icon">
          📥 导出CSV
        </button>
      </div>
    </div>

    <!-- 告警面板 -->
    <div v-if="recentAlerts.length > 0" class="alerts-panel">
      <h3>🚨 最近告警</h3>
      <div class="alerts-list">
        <div v-for="alert in recentAlerts" :key="alert.id" 
             :class="['alert-item', alertLevelClass(alert.level)]">
          <div class="alert-header">
            <span class="alert-level-badge">{{ alert.level }}</span>
            <span class="alert-time">{{ formatTime(alert.timestamp) }}</span>
          </div>
          <div class="alert-message">{{ alert.message }}</div>
          <div v-if="alert.metadata" class="alert-metadata">
            <span v-for="(value, key) in alert.metadata" :key="key" class="metadata-item">
              {{ key }}: {{ value }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <div class="metrics-grid">
      <!-- 实时统计卡片 -->
      <div class="stat-card">
        <div class="stat-icon">📈</div>
        <div class="stat-content">
          <div class="stat-value">{{ metrics.total_promotions || 0 }}</div>
          <div class="stat-label">总晋升数</div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon">📊</div>
        <div class="stat-content">
          <div class="stat-value">{{ (metrics.promotion_success_rate || 0).toFixed(1) }}%</div>
          <div class="stat-label">晋升成功率</div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon">⏱️</div>
        <div class="stat-content">
          <div class="stat-value">{{ metrics.current_queue_length || 0 }}</div>
          <div class="stat-label">当前队列长度</div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon">💾</div>
        <div class="stat-content">
          <div class="stat-value">{{ (metrics.cache_hit_rate || 0).toFixed(1) }}%</div>
          <div class="stat-label">缓存命中率</div>
        </div>
      </div>
    </div>

    <!-- 图表区域 -->
    <div class="charts-grid">
      <!-- 晋升趋势图 -->
      <div class="chart-card">
        <h3>📈 晋升趋势 (24小时)</h3>
        <canvas ref="promotionChart"></canvas>
      </div>

      <!-- 队列长度曲线 -->
      <div class="chart-card">
        <h3>📊 队列长度变化</h3>
        <canvas ref="queueChart"></canvas>
      </div>

      <!-- 分类分布饼图 -->
      <div class="chart-card">
        <h3>🥧 记忆分类分布</h3>
        <canvas ref="categoryChart"></canvas>
      </div>

      <!-- 信心等级分布 -->
      <div class="chart-card">
        <h3>🎯 信心等级分布</h3>
        <div class="confidence-bars">
          <div class="conf-bar">
            <div class="conf-label">高信心</div>
            <div class="conf-progress high">
              <div class="conf-fill" :style="{width: confidencePercent('high') + '%'}"></div>
            </div>
            <div class="conf-value">{{ metrics.high_confidence_count || 0 }}</div>
          </div>
          <div class="conf-bar">
            <div class="conf-label">中信心</div>
            <div class="conf-progress medium">
              <div class="conf-fill" :style="{width: confidencePercent('medium') + '%'}"></div>
            </div>
            <div class="conf-value">{{ metrics.medium_confidence_count || 0 }}</div>
          </div>
          <div class="conf-bar">
            <div class="conf-label">低信心</div>
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
      <h3>📋 详细统计</h3>
      <table class="metrics-table">
        <tr>
          <td>总晋升次数</td>
          <td class="value">{{ metrics.total_promotions || 0 }}</td>
          <td>总拒绝次数</td>
          <td class="value">{{ metrics.total_rejections || 0 }}</td>
        </tr>
        <tr>
          <td>总遗忘数量</td>
          <td class="value">{{ metrics.total_forgotten || 0 }}</td>
          <td>当前队列</td>
          <td class="value">{{ metrics.current_queue_length || 0 }}</td>
        </tr>
        <tr>
          <td>缓存命中</td>
          <td class="value">{{ metrics.cache_hits || 0 }}</td>
          <td>缓存未命中</td>
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
      // 生成CSV格式数据
      const csvData = this.generateCSV()
      const blob = new Blob([csvData], { type: 'text/csv;charset=utf-8;' })
      const link = document.createElement('a')
      const url = URL.createObjectURL(blob)
      link.setAttribute('href', url)
      link.setAttribute('download', `metrics_${new Date().toISOString().split('T')[0]}.csv`)
      link.style.visibility = 'hidden'
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
    },
    generateCSV() {
      const headers = '指标名称,数值,时间'
      const rows = [
        `总晋升数,${this.metrics.total_promotions || 0},${new Date().toISOString()}`,
        `总拒绝数,${this.metrics.total_rejections || 0},${new Date().toISOString()}`,
        `总遗忘数,${this.metrics.total_forgotten || 0},${new Date().toISOString()}`,
        `当前队列,${this.metrics.current_queue_length || 0},${new Date().toISOString()}`,
        `晋升成功率(%),${(this.metrics.promotion_success_rate || 0).toFixed(2)},${new Date().toISOString()}`,
        `缓存命中率(%),${(this.metrics.cache_hit_rate || 0).toFixed(2)},${new Date().toISOString()}`
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
        high: this.metrics.high_confidence_count || 0,
        medium: this.metrics.medium_confidence_count || 0,
        low: this.metrics.low_confidence_count || 0
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
            label: '晋升数量',
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
            label: '队列长度',
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
        'fact': '事实',
        'preference': '偏好',
        'goal': '目标',
        'noise': '噪音'
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
  max-width: 1400px;
  margin: 0 auto;
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
