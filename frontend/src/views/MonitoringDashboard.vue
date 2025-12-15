<template>
  <div class="monitoring-dashboard">
    <h1>📊 记忆系统监控中心</h1>

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
      refreshInterval: null
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
    this.refreshInterval = setInterval(() => this.loadMetrics(), 10000) // 每10秒刷新
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
