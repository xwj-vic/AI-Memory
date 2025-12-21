<template>
  <div class="alert-center">
    <!-- 统计卡片 -->
    <div class="stats-cards">
      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon error">🔴</div>
          <div class="stat-info">
            <div class="stat-value">{{ alertCounts.ERROR || 0 }}</div>
            <div class="stat-label">错误告警</div>
          </div>
        </div>
      </el-card>
      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon warning">🟡</div>
          <div class="stat-info">
            <div class="stat-value">{{ alertCounts.WARNING || 0 }}</div>
            <div class="stat-label">警告告警</div>
          </div>
        </div>
      </el-card>
      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon">📊</div>
          <div class="stat-info">
            <div class="stat-value">{{ stats.total_checks || 0 }}</div>
            <div class="stat-label">规则执行次数</div>
          </div>
        </div>
      </el-card>
      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon success">✅</div>
          <div class="stat-info">
            <div class="stat-value">{{ (stats.notify_success_rate * 100).toFixed(1) }}%</div>
            <div class="stat-label">通知成功率</div>
          </div>
        </div>
      </el-card>
    </div>

    <!-- 告警趋势图表 -->
    <el-card class="trend-card">
      <template #header>
        <div class="card-header">
          <h3>告警趋势（最近24小时）</h3>
          <el-button size="small" @click="fetchTrend">刷新</el-button>
        </div>
      </template>
      <div ref="chartRef" style="height: 300px;"></div>
    </el-card>

    <!-- 规则管理 -->
    <el-card class="rules-card">
      <template #header>
        <h3>规则管理</h3>
      </template>
      <el-table :data="rules" style="width: 100%">
        <el-table-column prop="name" label="规则名称" width="200" />
        <el-table-column prop="description" :label="$t('alerts.description')" min-width="180" />
        <el-table-column :label="$t('alerts.status')" width="80" align="center">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" @change="toggleRule(row.id, row.enabled)" />
          </template>
        </el-table-column>
        <el-table-column :label="$t('alerts.last_triggered')" width="180">
          <template #default="{ row }">{{ formatTime(row.stats?.last_fired_at) }}</template>
        </el-table-column>
        <el-table-column :label="$t('common.actions')" width="120" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openConfigDialog(row)">
              {{ $t('alerts.configure') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 实时告警列表 -->
    <el-card>
      <template #header>
        <div class="card-header">
          <h3>实时告警</h3>
          <div class="actions">
            <el-button type="primary" @click="dialogVisible = true">
              <el-icon><Plus /></el-icon> 手动创建
            </el-button>
          </div>
        </div>
      </template>

      <!-- Filters -->
      <div class="filters">
        <el-select v-model="filters.level" placeholder="筛选级别" clearable style="width: 150px">
          <el-option label="INFO" value="INFO" />
          <el-option label="WARNING" value="WARNING" />
          <el-option label="ERROR" value="ERROR" />
        </el-select>
        <el-input v-model="filters.rule" placeholder="筛选规则" clearable style="width: 200px" />
        <el-button @click="fetchAlerts">搜索</el-button>
      </div>

      <!-- Table -->
      <el-table :data="alerts" style="width: 100%" v-loading="loading">
        <el-table-column prop="timestamp" label="时间" width="180">
          <template #default="scope">
            {{ formatTime(scope.row.timestamp) }}
          </template>
        </el-table-column>
        <el-table-column prop="level" label="级别" width="100">
          <template #default="scope">
            <el-tag :type="getLevelType(scope.row.level)">{{ scope.row.level }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="rule" label="规则" width="150" />
        <el-table-column prop="message" label="消息" />
        <el-table-column label="操作" width="100">
          <template #default="scope">
            <el-button type="danger" circle size="small" @click="deleteAlert(scope.row.id)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- Pagination -->
      <div class="pagination">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.limit"
          :total="pagination.total"
          layout="prev, pager, next, sizes"
          @size-change="fetchAlerts"
          @current-change="fetchAlerts"
        />
      </div>
    </el-card>

    <!-- Create Dialog -->
    <el-dialog v-model="dialogVisible" title="手动创建告警" width="500px">
      <el-form :model="newAlert" label-width="80px">
        <el-form-item label="级别">
          <el-select v-model="newAlert.level">
            <el-option label="INFO" value="INFO" />
            <el-option label="WARNING" value="WARNING" />
            <el-option label="ERROR" value="ERROR" />
          </el-select>
        </el-form-item>
        <el-form-item label="规则">
          <el-input v-model="newAlert.rule" />
        </el-form-item>
        <el-form-item label="消息">
          <el-input v-model="newAlert.message" type="textarea" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="createAlert">确认</el-button>
      </template>
    </el-dialog>

    <!-- 规则配置对话框（合并基础和高级配置） -->
    <el-dialog v-model="editDialogVisible" title="规则配置" width="700px">
      <el-form v-if="editingRule" label-width="140px">
        <el-form-item label="规则名称">
          <span>{{ editingRule.name }}</span>
        </el-form-item>
        <el-form-item label="规则ID">
          <span style="color: #909399; font-size: 13px">{{ editingRule.id }}</span>
        </el-form-item>
        
        <el-divider content-position="left">基础配置</el-divider>
        
        <el-form-item label="冷却时间(分钟)">
          <el-input-number 
            v-model="editingRule.cooldown_minutes" 
            :min="1" 
            :max="1440"
            style="width: 200px"
          />
          <div style="margin-top: 8px; color: #909399; font-size: 12px">
            规则触发后的静默时间，避免频繁告警
          </div>
        </el-form-item>

        <el-divider content-position="left">高级配置 (JSON)</el-divider>
        
        <el-form-item label="配置JSON">
          <el-input
            v-model="editingRule.config_json_text"
            type="textarea"
            :rows="8"
            placeholder='{"threshold": 100}'
            style="font-family: monospace"
          />
          <div style="margin-top: 8px; color: #909399; font-size: 12px">
            JSON格式，如: {"threshold": 100, "window_minutes": 5}
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveRuleConfigCombined">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, reactive, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Plus } from '@element-plus/icons-vue'
import dayjs from 'dayjs'
import * as echarts from 'echarts'

const loading = ref(false)
const alerts = ref([])
const rules = ref([])
const stats = ref({})
const alertCounts = ref({})
const trendData = ref(null)
const chartRef = ref(null)
let chartInstance = null
const dialogVisible = ref(false)
const editDialogVisible = ref(false)
const jsonConfigDialogVisible = ref(false)
const editingRule = ref(null)

const filters = reactive({
  level: '',
  rule: ''
})

const pagination = reactive({
  page: 1,
  limit: 20,
  total: 0
})

const newAlert = reactive({
  level: 'INFO',
  rule: 'manual_test',
  message: 'Test Alert Message'
})

const formatTime = (time) => {
  if (!time) return '-'
  return dayjs(time).format('YYYY-MM-DD HH:mm:ss')
}

const getLevelType = (level) => {
  switch (level) {
    case 'ERROR': return 'danger'
    case 'WARNING': return 'warning'
    default: return 'info'
  }
}

// 获取统计信息
const fetchStats = async () => {
  try {
    const res = await fetch('/api/alerts/stats')
    const data = await res.json()
    stats.value = data
    alertCounts.value = data.by_level || {}
  } catch (err) {
    console.error('Failed to load stats:', err)
  }
}

// 获取规则列表
const fetchRules = async () => {
  try {
    const res = await fetch('/api/alerts/rules')
    const data = await res.json()
    rules.value = data.rules || []
  } catch (err) {
    ElMessage.error('Failed to load rules')
  }
}

// 获取告警趋势
const fetchTrend = async () => {
  try {
    const res = await fetch('/api/alerts/trend?hours=24')
    const data = await res.json()
    trendData.value = data
    renderChart()
  } catch (err) {
    console.error('Failed to load trend:', err)
  }
}

//  渲染图表
const renderChart = () => {
  if (!chartRef.value || !trendData.value) return

  if (!chartInstance) {
    chartInstance = echarts.init(chartRef.value)
  }

  const option = {
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'cross',
        label: {
          backgroundColor: '#6a7985'
        }
      }
    },
    legend: {
      data: ['错误', '警告', '信息'],
      top: 10
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      top: '15%',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: trendData.value.timestamps,
      axisLabel: {
        rotate: 45,
        fontSize: 11
      }
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      axisLabel: {
        formatter: '{value}'
      }
    },
    series: [
      {
        name: '错误',
        type: 'line',
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        lineStyle: {
          width: 2
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [{
              offset: 0, color: 'rgba(245, 108, 108, 0.3)'
            }, {
              offset: 1, color: 'rgba(245, 108, 108, 0.0)'
            }]
          }
        },
        data: trendData.value.error,
        itemStyle: { color: '#f56c6c' }
      },
      {
        name: '警告',
        type: 'line',
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        lineStyle: {
          width: 2
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [{
              offset: 0, color: 'rgba(230, 162, 60, 0.3)'
            }, {
              offset: 1, color: 'rgba(230, 162, 60, 0.0)'
            }]
          }
        },
        data: trendData.value.warning,
        itemStyle: { color: '#e6a23c' }
      },
      {
        name: '信息',
        type: 'line',
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        lineStyle: {
          width: 2
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [{
              offset: 0, color: 'rgba(144, 147, 153, 0.2)'
            }, {
              offset: 1, color: 'rgba(144, 147, 153, 0.0)'
            }]
          }
        },
        data: trendData.value.info,
        itemStyle: { color: '#909399' }
      }
    ]
  }

  chartInstance.setOption(option)
}

// 启用/禁用规则
const toggleRule = async (ruleID, enabled) => {
  try {
    await fetch(`/api/alerts/rules/${ruleID}/toggle`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled })
    })
    ElMessage.success(`规则已${enabled ? '启用' : '禁用'}`)
  } catch (err) {
    ElMessage.error('操作失败')
    fetchRules() // 重新加载
  }
}

// 打开配置对话框（合并基础和高级配置）
const openConfigDialog = (rule) => {
  editingRule.value = {
    ...rule,
    cooldown_minutes: parseInt(rule.cooldown / 60000000000), // nanoseconds to minutes
    config_json_text: rule.config_json || '{}'
  }
  editDialogVisible.value = true
}

// 保存规则配置（同时保存冷却时间和config_json）
const saveRuleConfigCombined = async () => {
  try {
    // 1. 验证JSON格式
    JSON.parse(editingRule.value.config_json_text)
    
    // 2. 保存冷却时间
    await fetch(`/api/alerts/rules/${editingRule.value.id}/config`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        cooldown_minutes: editingRule.value.cooldown_minutes
      })
    })
    
    // 3. 保存config_json
    await fetch(`/api/alerts/rules/${editingRule.value.id}/config-json`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        config_json: editingRule.value.config_json_text
      })
    })
    
    ElMessage.success('配置已更新')
    editDialogVisible.value = false
    fetchRules()
  } catch (err) {
    if (err instanceof SyntaxError) {
      ElMessage.error('JSON格式错误，请检查')
    } else {
      ElMessage.error('更新失败')
    }
  }
}

// 获取告警列表
const fetchAlerts = async () => {
  loading.value = true
  try {
    const params = new URLSearchParams({
      page: pagination.page,
      limit: pagination.limit,
      level: filters.level || '',
      rule: filters.rule || ''
    })
    const res = await fetch(`/api/alerts?${params}`)
    const data = await res.json()
    alerts.value = data.alerts || []
    pagination.total = data.total || 0
  } catch (err) {
    ElMessage.error('Failed to load alerts')
  } finally {
    loading.value = false
  }
}

// 删除告警
const deleteAlert = async (id) => {
  try {
    await ElMessageBox.confirm('确定删除此告警吗?', '警告', {
      type: 'warning'
    })
    await fetch(`/api/alerts/${id}`, { method: 'DELETE' })
    ElMessage.success('已删除')
    fetchAlerts()
  } catch (err) {
    // cancelled
  }
}

// 创建告警
const createAlert = async () => {
  try {
    await fetch('/api/alerts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(newAlert)
    })
    ElMessage.success('告警已创建')
    dialogVisible.value = false
    fetchAlerts()
    fetchStats()
  } catch (err) {
    ElMessage.error('创建失败')
  }
}

// 自动刷新
let refreshTimer = null
const startAutoRefresh = () => {
  refreshTimer = setInterval(() => {
    fetchStats()
    fetchTrend()
    fetchAlerts()
  }, 30000) // 30秒刷新一次
}

const stopAutoRefresh = () => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
}

onMounted(async () => {
  await Promise.all([
    fetchStats(),
    fetchRules(),
    fetchAlerts(),
    fetchTrend()
  ])
  await nextTick()
  renderChart()
  startAutoRefresh()
})

// 组件销毁时清理
import { onBeforeUnmount } from 'vue'
onBeforeUnmount(() => {
  stopAutoRefresh()
  if (chartInstance) {
    chartInstance.dispose()
  }
})
</script>

<style scoped>
.alert-center {
  padding: 20px;
}

.stats-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.stat-card {
  border-radius: 8px;
  transition: transform 0.2s;
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-icon {
  font-size: 36px;
  width: 60px;
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: #f5f5f5;
}

.stat-icon.error {
  background: #fef0f0;
}

.stat-icon.warning {
  background: #fdf6ec;
}

.stat-icon.success {
  background: #f0f9ff;
}

.stat-info {
  flex: 1;
}

.stat-value {
  font-size: 28px;
  font-weight: bold;
  color: #303133;
}

.stat-label {
  font-size: 14px;
  color: #909399;
  margin-top: 4px;
}

.trend-card, .rules-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header h3 {
  margin: 0;
  font-size: 16px;
}

.filters {
  margin-bottom: 20px;
  display: flex;
  gap: 10px;
}

.pagination {
  margin-top: 20px;
  text-align: right;
}
</style>
