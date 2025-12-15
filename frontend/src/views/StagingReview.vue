<template>
  <div class="staging-review">
    <div class="header">
      <h1>🔍 记忆审核中心</h1>
      <div class="stats">
        <div class="stat-card high">
          <div class="stat-value">{{ stats.high_confidence || 0 }}</div>
          <div class="stat-label">高信心</div>
        </div>
        <div class="stat-card medium">
          <div class="stat-value">{{ stats.medium_confidence || 0 }}</div>
          <div class="stat-label">待审核</div>
        </div>
        <div class="stat-card low">
          <div class="stat-value">{{ stats.low_confidence || 0 }}</div>
          <div class="stat-label">低信心</div>
        </div>
        <div class="stat-card total">
          <div class="stat-value">{{ stats.total_pending || 0 }}</div>
          <div class="stat-label">总计</div>
        </div>
      </div>
    </div>

    <div class="filters">
      <button 
        :class="['filter-btn', { active: filter === 'all' }]" 
        @click="filter = 'all'">
        全部
      </button>
      <button 
        :class="['filter-btn', { active: filter === 'pending' }]" 
        @click="filter = 'pending'">
        待审核
      </button>
      <button 
        :class="['filter-btn', { active: filter === 'high' }]" 
        @click="filter = 'high'">
        高信心
      </button>
    </div>

    <div v-if="loading" class="loading">加载中...</div>

    <div v-else-if="filteredEntries.length === 0" class="empty">
      <p>✨ 暂无待审核记忆</p>
    </div>

    <div v-else class="entries-list">
      <div 
        v-for="entry in filteredEntries" 
        :key="entry.id" 
        class="entry-card"
        :class="confidenceClass(entry.confidence_score)">
        <div class="entry-header">
          <span class="category-badge" :class="entry.category">
            {{ categoryLabels[entry.category] || entry.category }}
          </span>
          <span class="confidence-badge">
            信心: {{ (entry.confidence_score * 100).toFixed(0) }}%
          </span>
          <span class="occurrences-badge">
            出现 {{ entry.occurrence_count }} 次
          </span>
        </div>

        <div class="entry-content">{{ entry.content }}</div>

        <div class="entry-meta">
          <div class="tags">
            <span 
              v-for="tag in entry.extracted_tags" 
              :key="tag" 
              class="tag">
              #{{ tag }}
            </span>
          </div>
          <div class="entities" v-if="entry.extracted_entities && Object.keys(entry.extracted_entities).length > 0">
            <span 
              v-for="(value, key) in entry.extracted_entities" 
              :key="key"
              class="entity">
              {{ key }}: {{ value }}
            </span>
          </div>
        </div>

        <div class="entry-times">
          <span>首次: {{ formatTime(entry.first_seen_at) }}</span>
          <span>最近: {{ formatTime(entry.last_seen_at) }}</span>
        </div>

        <div class="entry-actions">
          <button 
            class="btn-confirm" 
            @click="confirmEntry(entry.id)"
            :disabled="processing">
            ✅ 确认晋升
          </button>
          <button 
            class="btn-reject" 
            @click="rejectEntry(entry.id)"
            :disabled="processing">
            ❌ 拒绝
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'StagingReview',
  data() {
    return {
      entries: [],
      stats: {},
      loading: true,
      processing: false,
      filter: 'all',
      categoryLabels: {
        'fact': '事实',
        'preference': '偏好',
        'goal': '目标',
        'noise': '噪音'
      }
    }
  },
  computed: {
    filteredEntries() {
      if (this.filter === 'all') return this.entries
      if (this.filter === 'pending') {
        return this.entries.filter(e => 
          e.confidence_score >= 0.5 && e.confidence_score < 0.8
        )
      }
      if (this.filter === 'high') {
        return this.entries.filter(e => e.confidence_score >= 0.8)
      }
      return this.entries
    }
  },
  mounted() {
    this.loadData()
    // 自动刷新
    this.refreshInterval = setInterval(() => this.loadData(), 30000)
  },
  beforeUnmount() {
    if (this.refreshInterval) {
      clearInterval(this.refreshInterval)
    }
  },
  methods: {
    async loadData() {
      try {
        // 获取统计
        const statsRes = await fetch('/api/staging/stats')
        this.stats = await statsRes.json()

        // 获取列表
        const entriesRes = await fetch('/api/staging')
        const data = await entriesRes.json()
        this.entries = data.entries || []
      } catch (error) {
        console.error('加载失败:', error)
      } finally {
        this.loading = false
      }
    },
    async confirmEntry(id) {
      if (!confirm('确认将此记忆晋升到长期记忆？')) return
      
      this.processing = true
      try {
        const res = await fetch(`/api/staging/${id}/confirm`, {
          method: 'POST'
        })
        if (res.ok) {
          this.entries = this.entries.filter(e => e.id !== id)
          this.loadData() // 刷新统计
        } else {
          alert('操作失败')
        }
      } catch (error) {
        console.error('确认失败:', error)
        alert('操作失败')
      } finally {
        this.processing = false
      }
    },
    async rejectEntry(id) {
      if (!confirm('确认拒绝此记忆？')) return
      
      this.processing = true
      try {
        const res = await fetch(`/api/staging/${id}/reject`, {
          method: 'POST'
        })
        if (res.ok) {
          this.entries = this.entries.filter(e => e.id !== id)
          this.loadData()
        } else {
          alert('操作失败')
        }
      } catch (error) {
        console.error('拒绝失败:', error)
        alert('操作失败')
      } finally {
        this.processing = false
      }
    },
    confidenceClass(score) {
      if (score >= 0.8) return 'high-conf'
      if (score >= 0.5) return 'medium-conf'
      return 'low-conf'
    },
    formatTime(timestamp) {
      if (!timestamp) return '-'
      const date = new Date(timestamp)
      const now = new Date()
      const diff = now - date
      const hours = Math.floor(diff / 3600000)
      if (hours < 24) return `${hours}小时前`
      const days = Math.floor(hours / 24)
      return `${days}天前`
    }
  }
}
</script>

<style scoped>
.staging-review {
  padding: 2rem;
  max-width: 1200px;
  margin: 0 auto;
}

.header h1 {
  margin: 0 0 1.5rem 0;
  font-size: 2rem;
  color: #2c3e50;
}

.stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 1rem;
  margin-bottom: 2rem;
}

.stat-card {
  background: white;
  border-radius: 12px;
  padding: 1.5rem;
  text-align: center;
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
  transition: transform 0.2s;
}

.stat-card:hover {
  transform: translateY(-2px);
}

.stat-card.high { border-left: 4px solid #10b981; }
.stat-card.medium { border-left: 4px solid #f59e0b; }
.stat-card.low { border-left: 4px solid #ef4444; }
.stat-card.total { border-left: 4px solid #3b82f6; }

.stat-value {
  font-size: 2rem;
  font-weight: bold;
  margin-bottom: 0.5rem;
}

.stat-label {
  color: #6b7280;
  font-size: 0.875rem;
}

.filters {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 2rem;
}

.filter-btn {
  padding: 0.5rem 1rem;
  border: 2px solid #e5e7eb;
  background: white;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}

.filter-btn:hover {
  border-color: #3b82f6;
}

.filter-btn.active {
  background: #3b82f6;
  color: white;
  border-color: #3b82f6;
}

.loading, .empty {
  text-align: center;
  padding: 3rem;
  color: #6b7280;
  font-size: 1.125rem;
}

.entries-list {
  display: grid;
  gap: 1rem;
}

.entry-card {
  background: white;
  border-radius: 12px;
  padding: 1.5rem;
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
  transition: all 0.2s;
  border-left: 4px solid #e5e7eb;
}

.entry-card.high-conf { border-left-color: #10b981; }
.entry-card.medium-conf { border-left-color: #f59e0b; }
.entry-card.low-conf { border-left-color: #ef4444; }

.entry-card:hover {
  box-shadow: 0 4px 12px rgba(0,0,0,0.15);
}

.entry-header {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1rem;
  flex-wrap: wrap;
}

.category-badge, .confidence-badge, .occurrences-badge {
  padding: 0.25rem 0.75rem;
  border-radius: 6px;
  font-size: 0.875rem;
  font-weight: 500;
}

.category-badge.fact { background: #dbeafe; color: #1e40af; }
.category-badge.preference { background: #fce7f3; color: #be185d; }
.category-badge.goal { background: #d1fae5; color: #065f46; }
.category-badge.noise { background: #fee2e2; color: #991b1b; }

.confidence-badge {
  background: #f3f4f6;
  color: #4b5563;
}

.occurrences-badge {
  background: #fef3c7;
  color: #92400e;
}

.entry-content {
  font-size: 1rem;
  line-height: 1.6;
  color: #1f2937;
  margin-bottom: 1rem;
  padding: 1rem;
  background: #f9fafb;
  border-radius: 8px;
}

.entry-meta {
  margin-bottom: 1rem;
}

.tags, .entities {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-top: 0.5rem;
}

.tag {
  background: #ede9fe;
  color: #5b21b6;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.875rem;
}

.entity {
  background: #fef3c7;
  color: #92400e;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.875rem;
}

.entry-times {
  font-size: 0.875rem;
  color: #6b7280;
  margin-bottom: 1rem;
  display: flex;
  gap: 1rem;
}

.entry-actions {
  display: flex;
  gap: 1rem;
}

.btn-confirm, .btn-reject {
  flex: 1;
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 8px;
  font-size: 1rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-confirm {
  background: #10b981;
  color: white;
}

.btn-confirm:hover:not(:disabled) {
  background: #059669;
}

.btn-reject {
  background: #ef4444;
  color: white;
}

.btn-reject:hover:not(:disabled) {
  background: #dc2626;
}

button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
