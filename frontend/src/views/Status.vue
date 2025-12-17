<script setup>
import { ref, onMounted } from 'vue'

const status = ref({})
const loading = ref(true)

const fetchStatus = async () => {
  try {
    const res = await fetch('/api/status')
    if (res.ok) {
      status.value = await res.json()
    }
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const getStatusType = (val) => {
  if (val === 'Online' || val === 'OK' || val === 'healthy') return 'success'
  if (val === 'Offline' || val === 'ERROR') return 'danger'
  return 'info'
}

onMounted(fetchStatus)
</script>

<template>
  <div class="status-page">
    <el-page-header>
      <template #content>
        <span class="page-title">{{ $t('status.title') }}</span>
      </template>
      <template #extra>
        <el-button @click="fetchStatus" :loading="loading" :icon="loading ? 'Loading' : 'Refresh'">
          {{ $t('common.refresh') }}
        </el-button>
      </template>
    </el-page-header>

    <el-row :gutter="16" style="margin-top: 24px;" v-loading="loading">
      <el-col :xs="24" :sm="12" :md="8" v-for="(val, key) in status" :key="key" style="margin-bottom: 16px;">
        <el-card shadow="hover">
          <el-statistic :title="String(key).toUpperCase()" :value="String(val)">
            <template #suffix>
              <el-tag :type="getStatusType(val)" size="small" style="margin-left: 8px;">
                {{ val === 'Online' ? $t('status.online') : val === 'Offline' ? $t('status.offline') : $t('status.normal') }}
              </el-tag>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
    </el-row>

    <el-empty v-if="!loading && Object.keys(status).length === 0" :description="$t('common.noData')" />
  </div>
</template>

<style scoped>
.status-page {
  padding: 24px;
  width: 100%;
  margin: 0;
}

.page-title {
  font-size: 24px;
  font-weight: 700;
}
</style>
