<template>
  <div class="admin-control">
    <el-page-header :icon="null">
      <template #content>
        <span class="page-title">{{ $t('control.title') }}</span>
      </template>
    </el-page-header>

    <el-row :gutter="24" class="control-sections">
      <!-- 手动触发区域 -->
      <el-col :span="24">
        <el-card shadow="hover">
          <template #header>
            <div class="card-header">
              <el-icon><Operation /></el-icon>
              <span>{{ $t('control.manualTrigger') }}</span>
            </div>
          </template>
          <p class="section-desc">{{ $t('control.manualTriggerDesc') }}</p>

          <el-row :gutter="16" class="trigger-cards">
            <!-- STM判定 -->
            <el-col :xs="24" :sm="12" :md="8">
              <el-card class="trigger-card" shadow="hover">
                <div class="card-icon">🔍</div>
                <h3>{{ $t('control.stmJudge') }}</h3>
                <p>{{ $t('control.stmJudgeDesc') }}</p>
                
                <el-form label-position="top" style="margin-top: 16px;">
                  <el-form-item label="User ID">
                    <el-input
                      v-model="judgeParams.userId"
                      placeholder="例: test_user"
                    />
                  </el-form-item>
                  <el-form-item label="Session ID">
                    <el-input
                      v-model="judgeParams.sessionId"
                      placeholder="例: session_1"
                    />
                  </el-form-item>
                </el-form>

                <el-button
                  type="primary"
                  @click="triggerJudge"
                  :loading="processing.judge"
                  style="width: 100%;"
                >
                  🚀 {{ processing.judge ? $t('common.processing') : $t('common.trigger') }}
                </el-button>

                <el-alert
                  v-if="results.judge"
                  :type="results.judge.success ? 'success' : 'error'"
                  :title="results.judge.message"
                  :closable="false"
                  style="margin-top: 12px;"
                />
              </el-card>
            </el-col>

            <!-- Staging晋升 -->
            <el-col :xs="24" :sm="12" :md="8">
              <el-card class="trigger-card" shadow="hover">
                <div class="card-icon">⬆️</div>
                <h3>{{ $t('control.stagingPromotion') }}</h3>
                <p>{{ $t('control.stagingPromotionDesc') }}</p>
                
                <el-button
                  type="success"
                  @click="triggerPromotion"
                  :loading="processing.promotion"
                  style="width: 100%; margin-top: 60px;"
                >
                  🎯 {{ processing.promotion ? $t('common.processing') : $t('common.trigger') }}
                </el-button>

                <el-alert
                  v-if="results.promotion"
                  :type="results.promotion.success ? 'success' : 'error'"
                  :title="results.promotion.message"
                  :closable="false"
                  style="margin-top: 12px;"
                />
              </el-card>
            </el-col>

            <!-- 遗忘扫描 -->
            <el-col :xs="24" :sm="12" :md="8">
              <el-card class="trigger-card" shadow="hover">
                <div class="card-icon">🗑️</div>
                <h3>{{ $t('control.decayScan') }}</h3>
                <p>{{ $t('control.decayScanDesc') }}</p>
                
                <el-button
                  type="warning"
                  @click="triggerDecay"
                  :loading="processing.decay"
                  style="width: 100%; margin-top: 60px;"
                >
                  🔄 {{ processing.decay ? $t('common.processing') : $t('common.trigger') }}
                </el-button>

                <el-alert
                  v-if="results.decay"
                  :type="results.decay.success ? 'success' : 'error'"
                  :title="results.decay.message"
                  :closable="false"
                  style="margin-top: 12px;"
                />
              </el-card>
            </el-col>
          </el-row>
        </el-card>
      </el-col>

      <!-- 系统状态 -->
      <el-col :span="24" style="margin-top: 24px;">
        <el-card shadow="hover">
          <template #header>
            <div class="card-header">
              <el-icon><DataAnalysis /></el-icon>
              <span>{{ $t('control.systemStatus') }}</span>
            </div>
          </template>
          
          <el-row :gutter="16">
            <el-col :xs="12" :sm="6">
              <el-statistic :title="$t('control.stmCount')" :value="systemStatus.stm_count || '-'" />
            </el-col>
            <el-col :xs="12" :sm="6">
              <el-statistic :title="$t('control.stagingCount')" :value="systemStatus.staging_count || 0" />
            </el-col>
            <el-col :xs="12" :sm="6">
              <el-statistic :title="$t('control.ltmCount')" :value="systemStatus.ltm_count || '-'" />
            </el-col>
            <el-col :xs="12" :sm="6">
              <el-statistic :title="$t('monitoring.totalPromotions')" :value="systemStatus.total_promotions || 0" />
            </el-col>
          </el-row>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script>
import { useI18n } from 'vue-i18n'

export default {
  name: 'AdminControl',
  setup() {
    const { t } = useI18n()
    return { t }
  },
  data() {
    return {
      judgeParams: {
        userId: 'test_user',
        sessionId: 'session_1'
      },
      processing: {
        judge: false,
        promotion: false,
        decay: false
      },
      processingAll: false,
      results: {
        judge: null,
        promotion: null,
        decay: null
      },
      systemStatus: {},
      refreshInterval: null
    }
  },
  mounted() {
    this.loadSystemStatus()
    this.refreshInterval = setInterval(() => this.loadSystemStatus(), 5000)
  },
  beforeUnmount() {
    if (this.refreshInterval) {
      clearInterval(this.refreshInterval)
    }
  },
  methods: {
    async triggerJudge() {
      if (!this.judgeParams.userId || !this.judgeParams.sessionId) {
        this.$message.warning('请填写 User ID 和 Session ID')
        return
      }

      this.processing.judge = true
      this.results.judge = null

      try {
        const res = await fetch('/api/admin/trigger-judge', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            user_id: this.judgeParams.userId,
            session_id: this.judgeParams.sessionId
          })
        })

        const data = await res.json()
        this.results.judge = {
          success: res.ok,
          message: data.message || data.error || '执行完成'
        }

        if (res.ok) {
          setTimeout(() => this.loadSystemStatus(), 1000)
        }
      } catch (error) {
        this.results.judge = {
          success: false,
          message: '请求失败: ' + error.message
        }
      } finally {
        this.processing.judge = false
      }
    },

    async triggerPromotion() {
      this.processing.promotion = true
      this.results.promotion = null

      try {
        const res = await fetch('/api/admin/trigger-promotion', {
          method: 'POST'
        })

        const data = await res.json()
        this.results.promotion = {
          success: res.ok,
          message: data.message || data.error || '执行完成'
        }

        if (res.ok) {
          setTimeout(() => this.loadSystemStatus(), 1000)
        }
      } catch (error) {
        this.results.promotion = {
          success: false,
          message: '请求失败: ' + error.message
        }
      } finally {
        this.processing.promotion = false
      }
    },

    async triggerDecay() {
      try {
        await this.$confirm('确认执行遗忘扫描？将删除衰减分数过低的记忆。', '提示', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        })
      } catch {
        return
      }

      this.processing.decay = true
      this.results.decay = null

      try {
        const res = await fetch('/api/admin/trigger-decay', {
          method: 'POST'
        })

        const data = await res.json()
        this.results.decay = {
          success: res.ok,
          message: data.message || data.error || '执行完成'
        }

        if (res.ok) {
          setTimeout(() => this.loadSystemStatus(), 1000)
        }
      } catch (error) {
        this.results.decay = {
          success: false,
          message: '请求失败: ' + error.message
        }
      } finally {
        this.processing.decay = false
      }
    },

    async loadSystemStatus() {
      try {
        const stagingRes = await fetch('/api/staging/stats')
        const stagingData = await stagingRes.json()

        const metricsRes = await fetch('/api/dashboard/metrics')
        const metricsData = await metricsRes.json()

        this.systemStatus = {
          staging_count: stagingData.total_pending || 0,
          total_promotions: metricsData.total_promotions || 0,
          stm_count: '≈6',
          ltm_count: '-'
        }
      } catch (error) {
        console.error('加载状态失败:', error)
      }
    }
  }
}
</script>

<style scoped>
.admin-control {
  padding: 24px;
  width: 100%;
  margin: 0;
}

.page-title {
  font-size: 24px;
  font-weight: 700;
}

.control-sections {
  margin-top: 24px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 16px;
  font-weight: 600;
}

.section-desc {
  color: #6b7280;
  margin-bottom: 20px;
}

.trigger-cards {
  margin-top: 16px;
}

.trigger-card {
  height: 100%;
  text-align: center;
}

.trigger-card:hover {
  transform: translateY(-4px);
  transition: all 0.3s;
}

.card-icon {
  font-size: 48px;
  margin-bottom: 16px;
}

.trigger-card h3 {
  margin: 0 0 12px 0;
  font-size: 18px;
  color: #1f2937;
}

.trigger-card p {
  color: #6b7280;
  font-size: 14px;
  margin-bottom: 16px;
  min-height: 60px;
}
</style>
