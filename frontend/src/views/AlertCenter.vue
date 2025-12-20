<template>
  <div class="alert-center">
    <el-card>
      <template #header>
        <div class="card-header">
          <h2>{{ $t('alerts.title') }}</h2>
          <div class="actions">
            <el-button type="primary" @click="dialogVisible = true">
              <el-icon><Plus /></el-icon> {{ $t('alerts.create') }}
            </el-button>
          </div>
        </div>
      </template>

      <!-- Filters -->
      <div class="filters">
        <el-select v-model="filters.level" :placeholder="$t('alerts.filterLevel')" clearable style="width: 150px">
          <el-option label="INFO" value="INFO" />
          <el-option label="WARNING" value="WARNING" />
          <el-option label="ERROR" value="ERROR" />
        </el-select>
        <el-input v-model="filters.rule" :placeholder="$t('alerts.filterRule')" clearable style="width: 200px" />
        <el-button @click="fetchAlerts">{{ $t('common.search') }}</el-button>
      </div>

      <!-- Table -->
      <el-table :data="alerts" style="width: 100%" v-loading="loading">
        <el-table-column prop="timestamp" :label="$t('common.time')" width="180">
          <template #default="scope">
            {{ formatTime(scope.row.timestamp) }}
          </template>
        </el-table-column>
        <el-table-column prop="level" :label="$t('alerts.level')" width="100">
          <template #default="scope">
            <el-tag :type="getLevelType(scope.row.level)">{{ scope.row.level }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="rule" :label="$t('alerts.rule')" width="150" />
        <el-table-column prop="message" :label="$t('alerts.message')" />
        <el-table-column :label="$t('common.actions')" width="100">
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
    <el-dialog v-model="dialogVisible" :title="$t('alerts.createTitle')" width="500px">
      <el-form :model="newAlert" label-width="80px">
        <el-form-item :label="$t('alerts.level')">
          <el-select v-model="newAlert.level">
            <el-option label="INFO" value="INFO" />
            <el-option label="WARNING" value="WARNING" />
            <el-option label="ERROR" value="ERROR" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('alerts.rule')">
          <el-input v-model="newAlert.rule" />
        </el-form-item>
        <el-form-item :label="$t('alerts.message')">
          <el-input v-model="newAlert.message" type="textarea" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="createAlert">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, reactive } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Plus } from '@element-plus/icons-vue'
import dayjs from 'dayjs'

const loading = ref(false)
const alerts = ref([])
const dialogVisible = ref(false)

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

const formatTime = (time) => dayjs(time).format('YYYY-MM-DD HH:mm:ss')

const getLevelType = (level) => {
  switch (level) {
    case 'ERROR': return 'danger'
    case 'WARNING': return 'warning'
    default: return 'info'
  }
}

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

const deleteAlert = async (id) => {
  try {
    await ElMessageBox.confirm('Are you sure to delete this alert?', 'Warning', {
      type: 'warning'
    })
    await fetch(`/api/alerts/${id}`, { method: 'DELETE' })
    ElMessage.success('Alert deleted')
    fetchAlerts()
  } catch (err) {
    // cancelled
  }
}

const createAlert = async () => {
  try {
    await fetch('/api/alerts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(newAlert)
    })
    ElMessage.success('Alert created')
    dialogVisible.value = false
    fetchAlerts()
  } catch (err) {
    ElMessage.error('Failed to create alert')
  }
}

onMounted(fetchAlerts)
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
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
