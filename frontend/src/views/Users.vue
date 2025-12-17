<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

const router = useRouter()
const users = ref([])
const loading = ref(true)

const fetchUsers = async () => {
  try {
    const res = await fetch('/api/users')
    if (res.ok) {
      const data = await res.json()
      users.value = data.users || []
    } else {
      ElMessage.error('加载用户列表失败')
    }
  } catch (e) {
    console.error(e)
    ElMessage.error('请求失败')
  } finally {
    loading.value = false
  }
}

const formatDate = (dateStr) => {
  if (!dateStr) return 'Never'
  return new Date(dateStr).toLocaleString('zh-CN')
}

const viewMemories = (userId) => {
  router.push(`/admin/memory?userId=${userId}`)
}

onMounted(fetchUsers)
</script>

<template>
  <div class="users-page">
    <el-page-header>
      <template #content>
        <span class="page-title">{{ $t('users.title') }}</span>
      </template>
      <template #extra>
        <el-button @click="fetchUsers" :icon="loading ? 'Loading' : 'Refresh'" :loading="loading">
          {{ $t('common.refresh') }}
        </el-button>
      </template>
    </el-page-header>

    <el-card shadow="hover" style="margin-top: 24px;">
      <el-table
        :data="users"
        v-loading="loading"
        stripe
        style="width: 100%;"
        :empty-text="'暂无用户数据'"
      >
        <el-table-column prop="user_identifier" :label="$t('users.userId')" min-width="200">
          <template #default="{ row }">
            <el-text type="primary" tag="code">{{ row.user_identifier }}</el-text>
          </template>
        </el-table-column>

        <el-table-column prop="last_active" :label="$t('users.lastActive')" min-width="180">
          <template #default="{ row }">
            <el-text>{{ formatDate(row.last_active) }}</el-text>
          </template>
        </el-table-column>

        <el-table-column prop="session_count" :label="$t('users.sessionCount')" min-width="120" align="center">
          <template #default="{ row }">
            <el-tag type="info" size="large">{{ row.session_count }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column prop="ltm_count" :label="$t('users.ltmCount')" min-width="120" align="center">
          <template #default="{ row }">
            <el-tag type="success" size="large">{{ row.ltm_count }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column :label="$t('users.actions')" min-width="120" align="center">
          <template #default="{ row }">
            <el-button
              type="primary"
              size="small"
              @click="viewMemories(row.user_identifier)"
            >
              {{ $t('memory.viewMemories') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<style scoped>
.users-page {
  padding: 24px;
  width: 100%;
  margin: 0;
}

.page-title {
  font-size: 24px;
  font-weight: 700;
}
</style>
