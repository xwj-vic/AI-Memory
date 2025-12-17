<template>
  <div class="staging-review">
    <el-page-header>
      <template #content>
        <span class="page-title">{{ $t('staging.title') }}</span>
      </template>
    </el-page-header>

    <!-- 统计卡片 -->
    <el-row :gutter="16" style="margin-top: 24px;">
      <el-col :xs="6" :sm="6">
        <el-card shadow="hover" class="stat-card">
          <el-statistic :title="$t('staging.highConfidence')" :value="stats.high_confidence || 0">
            <template #prefix>
              <el-icon color="#10b981"><CircleCheck /></el-icon>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
      <el-col :xs="6" :sm="6">
        <el-card shadow="hover" class="stat-card">
          <el-statistic :title="$t('staging.pending')" :value="stats.medium_confidence || 0">
            <template #prefix>
              <el-icon color="#f59e0b"><Clock /></el-icon>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
      <el-col :xs="6" :sm="6">
        <el-card shadow="hover" class="stat-card">
          <el-statistic :title="$t('staging.lowConfidence')" :value="stats.low_confidence || 0">
            <template #prefix>
              <el-icon color="#ef4444"><Warning /></el-icon>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
      <el-col :xs="6" :sm="6">
        <el-card shadow="hover" class="stat-card">
          <el-statistic :title="$t('staging.total')" :value="stats.total_pending || 0">
            <template #prefix>
              <el-icon color="#3b82f6"><DataAnalysis /></el-icon>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
    </el-row>

    <!-- 筛选器 -->
    <el-radio-group v-model="filter" style="margin: 24px 0;">
      <el-radio-button label="all">{{ $t('common.all') }}</el-radio-button>
      <el-radio-button label="pending">{{ $t('staging.pending') }}</el-radio-button>
      <el-radio-button label="high">{{ $t('staging.highConfidence') }}</el-radio-button>
    </el-radio-group>

    <!-- 内容区域 -->
    <div v-loading="loading">
      <el-empty v-if="!loading && filteredEntries.length === 0" :description="$t('staging.noMemories')" />

      <el-row :gutter="16" v-else>
        <el-col 
          :xs="24" 
          :sm="12" 
          :md="8"
          v-for="entry in filteredEntries" 
          :key="entry.id"
          style="margin-bottom: 16px;"
        >
          <el-card shadow="hover" class="entry-card">
            <!-- 头部信息 -->
            <template #header>
              <div class="card-header">
                <el-tag :type="getCategoryType(entry.category)" size="small">
                  {{ $t(`staging.categories.${entry.category}`) || entry.category }}
                </el-tag>
                <el-tag size="small">
                  {{ $t('staging.confidence') }}: {{ (entry.confidence_score * 100).toFixed(0) }}%
                </el-tag>
                <el-tag type="warning" size="small">
                  {{ entry.occurrence_count }} {{ $t('staging.times') }}
                </el-tag>
              </div>
            </template>

            <!-- 内容 -->
            <div class="entry-content">{{ entry.content }}</div>

            <!-- 标签和实体 -->
            <div class="entry-meta">
              <div v-if="entry.extracted_tags && entry.extracted_tags.length > 0" style="margin-bottom: 8px;">
                <el-tag 
                  v-for="tag in entry.extracted_tags" 
                  :key="tag"
                  type="info"
                  size="small"
                  style="margin-right: 4px; margin-bottom: 4px;"
                >
                  #{{ tag }}
                </el-tag>
              </div>
              <div v-if="entry.extracted_entities && Object.keys(entry.extracted_entities).length > 0">
                <el-tag 
                  v-for="(value, key) in entry.extracted_entities" 
                  :key="key"
                  type="warning"
                  size="small"
                  style="margin-right: 4px; margin-bottom: 4px;"
                >
                  {{ key }}: {{ value }}
                </el-tag>
              </div>
            </div>

            <!-- 时间信息 -->
            <el-descriptions :column="2" size="small" style="margin-top: 12px;">
              <el-descriptions-item :label="$t('staging.firstSeen')">{{ formatTime(entry.first_seen_at) }}</el-descriptions-item>
              <el-descriptions-item :label="$t('staging.lastSeen')">{{ formatTime(entry.last_seen_at) }}</el-descriptions-item>
            </el-descriptions>

            <!-- 操作按钮 -->
            <template #footer>
              <el-button
                type="success"
                @click="confirmEntry(entry.id)"
                :loading="processing"
                size="small"
              >
                ✅ {{ $t('staging.confirmPromotion') }}
              </el-button>
              <el-button
                type="danger"
                @click="rejectEntry(entry.id)"
                :loading="processing"
                size="small"
              >
                ❌ {{ $t('staging.reject') }}
              </el-button>
            </template>
          </el-card>
        </el-col>
      </el-row>
    </div>
  </div>
</template>

<script>
import { ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'

export default {
  name: 'StagingReview',
  setup() {
    const { t } = useI18n()
    return { t }
  },
  data() {
    return {
      entries: [],
      stats: {},
      loading: true,
      processing: false,
      filter: 'all',
      categoryLabels: {
        'fact': 'fact',
        'preference': 'preference',
        'goal': 'goal',
        'noise': 'noise'
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
        const statsRes = await fetch('/api/staging/stats')
        this.stats = await statsRes.json()

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
      try {
        await ElMessageBox.confirm(this.t('staging.confirmPromotionMsg'), this.t('common.confirm'), {
          confirmButtonText: this.t('common.confirm'),
          cancelButtonText: this.t('common.cancel'),
          type: 'success'
        })
      } catch {
        return
      }
      
      this.processing = true
      try {
        const res = await fetch(`/api/staging/${id}/confirm`, {
          method: 'POST'
        })
        if (res.ok) {
          this.entries = this.entries.filter(e => e.id !== id)
          this.loadData()
          this.$message.success(this.t('staging.promotionSuccess'))
        } else {
          this.$message.error(this.t('common.error'))
        }
      } catch (error) {
        console.error('确认失败:', error)
        this.$message.error(this.t('common.error'))
      } finally {
        this.processing = false
      }
    },
    async rejectEntry(id) {
      try {
        await ElMessageBox.confirm(this.t('staging.rejectConfirmMsg'), this.t('common.confirm'), {
          confirmButtonText: this.t('common.confirm'),
          cancelButtonText: this.t('common.cancel'),
          type: 'warning'
        })
      } catch {
        return
      }
      
      this.processing = true
      try {
        const res = await fetch(`/api/staging/${id}/reject`, {
          method: 'POST'
        })
        if (res.ok) {
          this.entries = this.entries.filter(e => e.id !== id)
          this.loadData()
          this.$message.success(this.t('staging.rejected'))
        } else {
          this.$message.error(this.t('common.error'))
        }
      } catch (error) {
        console.error('拒绝失败:', error)
        this.$message.error(this.t('common.error'))
      } finally {
        this.processing = false
      }
    },
    getCategoryType(category) {
      const types = {
        'fact': 'primary',
        'preference': 'success',
        'goal': 'warning',
        'noise': 'danger'
      }
      return types[category] || 'info'
    },
    formatTime(timestamp) {
      if (!timestamp) return '-'
      const date = new Date(timestamp)
      const now = new Date()
      const diff = now - date
      const hours = Math.floor(diff / 3600000)
      if (hours < 24) return `${hours}${this.t('staging.times')}`
      const days = Math.floor(hours / 24)
      return `${days}天前`
    }
  }
}
</script>

<style scoped>
.staging-review {
  padding: 24px;
  width: 100%;
  margin: 0;
}

.page-title {
  font-size: 24px;
  font-weight: 700;
}

.stat-card:hover {
  transform: translateY(-2px);
  transition: all 0.3s;
}

.entry-card {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.card-header {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.entry-content {
  font-size: 14px;
  line-height: 1.6;
  margin-bottom: 12px;
  color: #1f2937;
}

.entry-meta {
  margin-bottom: 12px;
}
</style>
