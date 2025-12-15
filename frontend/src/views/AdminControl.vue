<template>
  <div class="admin-control">
    <h1>🎛️ 系统管理控制台</h1>

    <div class="control-sections">
      <!-- 手动触发区域 -->
      <div class="section">
        <h2>⚡ 手动触发任务</h2>
        <p class="section-desc">手动执行漏斗型记忆系统的后台任务</p>

        <div class="trigger-cards">
          <!-- STM判定 -->
          <div class="trigger-card">
            <div class="card-icon">🔍</div>
            <div class="card-content">
              <h3>STM判定流程</h3>
              <p>对短期记忆进行LLM价值判定，符合条件的进入Staging暂存区</p>
              <div class="input-group">
                <input 
                  v-model="judgeParams.userId" 
                  placeholder="User ID (例: test_user)"
                  class="input-field">
                <input 
                  v-model="judgeParams.sessionId" 
                  placeholder="Session ID (例: session_1)"
                  class="input-field">
              </div>
              <button 
                @click="triggerJudge" 
                :disabled="processing.judge"
                class="btn btn-primary">
                {{ processing.judge ? '处理中...' : '🚀 触发判定' }}
              </button>
              <div v-if="results.judge" :class="['result', results.judge.success ? 'success' : 'error']">
                {{ results.judge.message }}
              </div>
            </div>
          </div>

          <!-- Staging晋升 -->
          <div class="trigger-card">
            <div class="card-icon">⬆️</div>
            <div class="card-content">
              <h3>Staging晋升流程</h3>
              <p>扫描暂存区，将满足条件的记忆晋升到长期记忆（LTM）</p>
              <button 
                @click="triggerPromotion" 
                :disabled="processing.promotion"
                class="btn btn-success">
                {{ processing.promotion ? '处理中...' : '🎯 触发晋升' }}
              </button>
              <div v-if="results.promotion" :class="['result', results.promotion.success ? 'success' : 'error']">
                {{ results.promotion.message }}
              </div>
            </div>
          </div>

          <!-- 遗忘扫描 -->
          <div class="trigger-card">
            <div class="card-icon">🗑️</div>
            <div class="card-content">
              <h3>遗忘扫描</h3>
              <p>扫描长期记忆，删除衰减分数过低的记忆（自动遗忘机制）</p>
              <button 
                @click="triggerDecay" 
                :disabled="processing.decay"
                class="btn btn-warning">
                {{ processing.decay ? '处理中...' : '🔄 触发扫描' }}
              </button>
              <div v-if="results.decay" :class="['result', results.decay.success ? 'success' : 'error']">
                {{ results.decay.message }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 快捷操作 -->
      <div class="section">
        <h2>⚡ 快捷操作</h2>
        <div class="quick-actions">
          <button @click="runFullCycle" :disabled="processingAll" class="btn btn-large">
            {{ processingAll ? '执行中...' : '🔁 执行完整周期 (判定→晋升→遗忘)' }}
          </button>
          <button @click="viewLogs" class="btn btn-secondary btn-large">
            📋 查看系统日志
          </button>
        </div>
      </div>

      <!-- 系统状态 -->
      <div class="section">
        <h2>📊 实时状态</h2>
        <div class="status-grid">
          <div class="status-card">
            <div class="status-label">STM记忆数</div>
            <div class="status-value">{{ systemStatus.stm_count || '-' }}</div>
          </div>
          <div class="status-card">
            <div class="status-label">Staging队列</div>
            <div class="status-value">{{ systemStatus.staging_count || 0 }}</div>
          </div>
          <div class="status-card">
            <div class="status-label">LTM记忆数</div>
            <div class="status-value">{{ systemStatus.ltm_count || '-' }}</div>
          </div>
          <div class="status-card">
            <div class="status-label">总晋升数</div>
            <div class="status-value">{{ systemStatus.total_promotions || 0 }}</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'AdminControl',
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
        alert('请填写 User ID 和 Session ID')
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
      if (!confirm('确认执行遗忘扫描？将删除衰减分数过低的记忆。')) {
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

    async runFullCycle() {
      if (!confirm('执行完整周期：判定→晋升→遗忘，可能需要较长时间，确认？')) {
        return
      }

      this.processingAll = true

      try {
        // 1. 判定
        await this.triggerJudge()
        await new Promise(resolve => setTimeout(resolve, 2000))

        // 2. 晋升
        await this.triggerPromotion()
        await new Promise(resolve => setTimeout(resolve, 2000))

        // 3. 遗忘
        await this.triggerDecay()

        alert('✅ 完整周期执行完成！')
      } catch (error) {
        alert('❌ 执行出错: ' + error.message)
      } finally {
        this.processingAll = false
        this.loadSystemStatus()
      }
    },

    async loadSystemStatus() {
      try {
        // 获取Staging统计
        const stagingRes = await fetch('/api/staging/stats')
        const stagingData = await stagingRes.json()

        // 获取Dashboard指标
        const metricsRes = await fetch('/api/dashboard/metrics')
        const metricsData = await metricsRes.json()

        this.systemStatus = {
          staging_count: stagingData.total_pending || 0,
          total_promotions: metricsData.total_promotions || 0,
          stm_count: '≈6', // 这个需要额外API
          ltm_count: '-'   // 这个需要额外API
        }
      } catch (error) {
        console.error('加载状态失败:', error)
      }
    },

    viewLogs() {
      // 打开新标签查看日志（需要后端支持）
      alert('日志功能开发中...\n当前可查看终端输出或 /tmp/ai-memory.log')
    }
  }
}
</script>

<style scoped>
.admin-control {
  padding: 2rem;
  max-width: 1400px;
  margin: 0 auto;
}

h1 {
  margin-bottom: 2rem;
  color: #1f2937;
}

.control-sections {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

.section {
  background: white;
  border-radius: 12px;
  padding: 2rem;
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
}

.section h2 {
  margin: 0 0 0.5rem 0;
  color: #374151;
  font-size: 1.5rem;
}

.section-desc {
  color: #6b7280;
  margin-bottom: 1.5rem;
}

.trigger-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 1.5rem;
}

.trigger-card {
  border: 2px solid #e5e7eb;
  border-radius: 12px;
  padding: 1.5rem;
  transition: all 0.2s;
}

.trigger-card:hover {
  border-color: #3b82f6;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.1);
}

.card-icon {
  font-size: 3rem;
  text-align: center;
  margin-bottom: 1rem;
}

.card-content h3 {
  margin: 0 0 0.5rem 0;
  color: #1f2937;
  font-size: 1.25rem;
}

.card-content p {
  color: #6b7280;
  font-size: 0.875rem;
  margin-bottom: 1rem;
  min-height: 3rem;
}

.input-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.input-field {
  padding: 0.75rem;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 0.875rem;
}

.input-field:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.btn {
  width: 100%;
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 8px;
  font-size: 1rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-primary {
  background: #3b82f6;
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background: #2563eb;
}

.btn-success {
  background: #10b981;
  color: white;
}

.btn-success:hover:not(:disabled) {
  background: #059669;
}

.btn-warning {
  background: #f59e0b;
  color: white;
}

.btn-warning:hover:not(:disabled) {
  background: #d97706;
}

.btn-secondary {
  background: #6b7280;
  color: white;
}

.btn-secondary:hover:not(:disabled) {
  background: #4b5563;
}

.btn-large {
  padding: 1rem 2rem;
  font-size: 1.125rem;
}

.result {
  margin-top: 1rem;
  padding: 0.75rem;
  border-radius: 6px;
  font-size: 0.875rem;
}

.result.success {
  background: #d1fae5;
  color: #065f46;
  border: 1px solid #10b981;
}

.result.error {
  background: #fee2e2;
  color: #991b1b;
  border: 1px solid #ef4444;
}

.quick-actions {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
}

.quick-actions .btn {
  flex: 1;
  min-width: 200px;
}

.status-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 1rem;
}

.status-card {
  background: #f9fafb;
  border-radius: 8px;
  padding: 1.5rem;
  text-align: center;
  border: 1px solid #e5e7eb;
}

.status-label {
  font-size: 0.875rem;
  color: #6b7280;
  margin-bottom: 0.5rem;
}

.status-value {
  font-size: 2rem;
  font-weight: bold;
  color: #1f2937;
}
</style>
