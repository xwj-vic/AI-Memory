<script setup>
import { ref, onMounted, reactive, watch, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const memories = ref([])
const router = useRouter()
const route = useRoute()
const user = JSON.parse(localStorage.getItem('user') || '{}')

const activeTab = ref('long_term')
const filters = reactive({
  userId: '',
})
const page = ref(1)
const limit = ref(50)
const selectedMemory = ref(null)
const dialogVisible = ref(false)

onMounted(() => {
  if (route.query.userId) {
    filters.userId = route.query.userId
  }
  fetchMemories()
})

const fetchMemories = async () => {
  try {
    const query = new URLSearchParams({
      user_id: filters.userId,
      type: activeTab.value,
      page: page.value,
      limit: limit.value
    }).toString()

    const res = await fetch(`/api/memories?${query}`)
    if (res.ok) {
      const data = await res.json()
      memories.value = data.memories || []
    } else {
      if (res.status === 401) router.push('/login')
    }
  } catch (e) {
    console.error(e)
  }
}

watch(activeTab, () => {
  page.value = 1
  fetchMemories()
})

const groupedShortTerm = computed(() => {
  if (activeTab.value !== 'short_term') return {}
  
  const groups = {}
  memories.value.forEach(mem => {
    const sessionId = mem.metadata?.session_id || 'Unknown Session'
    if (!groups[sessionId]) {
      groups[sessionId] = []
    }
    groups[sessionId].push(mem)
  })
  return groups
})

const deleteMemory = async (id) => {
  try {
    await ElMessageBox.confirm(t('memory.deleteConfirm'), t('common.confirm'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning'
    })
  } catch {
    return
  }
  
  try {
    const res = await fetch(`/api/memories/${id}`, { method: 'DELETE' })
    if (res.ok) {
      ElMessage.success(t('memory.deleteSuccess'))
      fetchMemories()
    }
  } catch (e) {
    ElMessage.error(t('common.error'))
  }
}

const search = () => {
  page.value = 1
  fetchMemories()
}

const isEditing = ref(false)
const editContent = ref('')

const openModal = (mem) => {
  selectedMemory.value = mem
  isEditing.value = false
  editContent.value = mem.content
  dialogVisible.value = true
}

const closeModal = () => {
  dialogVisible.value = false
  selectedMemory.value = null
  isEditing.value = false
}

const updateMemory = async () => {
  if (!selectedMemory.value) return
  try {
    const res = await fetch(`/api/memories/${selectedMemory.value.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ content: editContent.value })
    })
    if (res.ok) {
      ElMessage.success(t('memory.updateSuccess'))
      selectedMemory.value.content = editContent.value
      isEditing.value = false
      fetchMemories()
    } else {
      ElMessage.error(t('common.error'))
    }
  } catch (e) {
    console. error(e)
    ElMessage.error(t('common.error'))
  }
}
</script>

<template>
  <div class="memory-explorer">
    <el-page-header :icon="null">
      <template #content>
        <span class="page-title">{{ $t('memory.title') }}</span>
      </template>
      <template #extra>
        <span style="margin-right: 16px; color: #6b7280;">{{ $t('common.loggedInAs') }}: {{ user.username }}</span>
      </template>
    </el-page-header>

    <!-- 搜索栏 -->
    <el-card shadow="hover" style="margin-top: 24px;">
      <el-row :gutter="16">
        <el-col :span="18">
          <el-input
            v-model="filters.userId"
            :placeholder="$t('memory.filterByUser')"
            clearable
          />
        </el-col>
        <el-col :span="6">
          <el-button type="primary" @click="search" style="width: 100%;">
            {{ $t('common.search') }}
          </el-button>
        </el-col>
      </el-row>
    </el-card>

    <!-- 标签页 -->
    <el-tabs v-model="activeTab" style="margin-top: 24px;">
      <el-tab-pane :label="$t('memory.longTerm')" name="long_term">
        <el-empty v-if="memories.length === 0" :description="$t('common.noData')" />
        
        <el-row :gutter="16" v-else>
          <el-col 
            :xs="24" 
            :sm="12" 
            :md="8" 
            :lg="6"
            v-for="mem in memories" 
            :key="mem.id"
            style="margin-bottom: 16px;"
          >
            <el-card shadow="hover" class="memory-card" @click="openModal(mem)">
              <template #header>
                <div class="card-header">
                  <el-tag type="success" size="small">LTM</el-tag>
                  <el-text size="small" type="info">
                    {{ new Date(mem.timestamp).toLocaleDateString() }}
                  </el-text>
                </div>
              </template>
              
              <el-text line-clamp="4" class="memory-text">
                {{ mem.content }}
              </el-text>
              
              <template #footer>
                <div class="card-footer">
                  <el-text size="small" type="info">
                    {{ $t('memory.userId') }}: {{ mem.metadata?.user_id || 'N/A' }}
                  </el-text>
                  <el-button
                    type="danger"
                    size="small"
                    :icon="'Delete'"
                    circle
                    @click.stop="deleteMemory(mem.id)"
                  />
                </div>
              </template>
            </el-card>
          </el-col>
        </el-row>
      </el-tab-pane>

      <el-tab-pane :label="$t('memory.shortTerm')" name="short_term">
        <el-empty v-if="memories.length === 0" :description="$t('common.noData')" />
        
        <div v-else>
          <el-collapse accordion>
            <el-collapse-item 
              v-for="(sessionMemories, sessionId) in groupedShortTerm" 
              :key="sessionId"
              :name="sessionId"
            >
              <template #title>
                <el-icon><Folder /></el-icon>
                <span style="margin-left: 8px;">{{ $t('memory.session') }}: {{ sessionId }}</span>
                <el-tag size="small" style="margin-left: 12px;">{{ sessionMemories.length }} {{ $t('memory.items') }}</el-tag>
              </template>
              
              <div v-for="mem in sessionMemories" :key="mem.id" class="session-item">
                <div class="session-header">
                  <el-text size="small" type="info">
                    {{ new Date(mem.timestamp).toLocaleTimeString() }}
                  </el-text>
                  <el-button
                    type="danger"
                    size="small"
                    link
                    @click="deleteMemory(mem.id)"
                  >
                    {{ $t('common.delete') }}
                  </el-button>
                </div>
                <div @click="openModal(mem)" style="cursor: pointer;">
                  <el-text>{{ mem.content }}</el-text>
                </div>
              </div>
            </el-collapse-item>
          </el-collapse>
        </div>
      </el-tab-pane>
    </el-tabs>

    <!-- 分页 -->
    <el-pagination
      v-if="memories.length > 0"
      v-model:current-page="page"
      :page-size="limit"
      layout="prev, pager, next"
      :total="page * limit"
      @current-change="fetchMemories"
      style="margin-top: 24px; justify-content: center;"
    />

    <!-- 详情对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="$t('memory.details')"
      width="600px"
      @close="closeModal"
    >
      <div v-if="selectedMemory">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="ID">
            <el-text tag="code">{{ selectedMemory.id }}</el-text>
          </el-descriptions-item>
          <el-descriptions-item :label="$t('memory.type')">
            <el-tag :type="selectedMemory.type === 'long_term' ? 'success' : 'primary'">
              {{ selectedMemory.type }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="$t('memory.time')">
            {{ new Date(selectedMemory.timestamp).toLocaleString() }}
          </el-descriptions-item>
        </el-descriptions>

        <el-divider />

        <div style="margin-bottom: 16px;">
          <label style="font-weight: 500; display: block; margin-bottom: 8px;">{{ $t('memory.content') }}</label>
          <el-input
            v-if="isEditing"
            v-model="editContent"
            type="textarea"
            :rows="6"
          />
          <el-text v-else style="white-space: pre-wrap;">{{ selectedMemory.content }}</el-text>
        </div>

        <div>
          <label style="font-weight: 500; display: block; margin-bottom: 8px;">{{ $t('memory.metadata') }}</label>
          <el-text tag="pre" style="background: #f5f5f5; padding: 12px; border-radius: 4px; overflow-x: auto;">{{
            JSON.stringify(selectedMemory.metadata, null, 2)
          }}</el-text>
        </div>
      </div>

      <template #footer>
        <el-button v-if="!isEditing" type="primary" @click="isEditing = true">{{ $t('common.edit') }}</el-button>
        <el-button v-if="isEditing" type="success" @click="updateMemory">{{ $t('common.save') }}</el-button>
        <el-button @click="closeModal">{{ $t('common.close') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.memory-explorer {
  padding: 24px;
  width: 100%;
  margin: 0;
}

.page-title {
  font-size: 24px;
  font-weight: 700;
}

.memory-card {
  cursor: pointer;
  height: 100%;
  transition: all 0.3s;
}

.memory-card:hover {
  transform: translateY(-4px);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.memory-text {
  display: block;
  margin: 12px 0;
}

.card-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.session-item {
  padding: 12px;
  border-bottom: 1px solid #f0f0f0;
}

.session-item:last-child {
  border-bottom: none;
}

.session-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
</style>
