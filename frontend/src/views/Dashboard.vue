<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'

const memories = ref([])
const router = useRouter()
const user = JSON.parse(localStorage.getItem('user') || '{}')

const fetchMemories = async () => {
  try {
    const res = await fetch('/api/memories')
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

const deleteMemory = async (id) => {
    if (!confirm('Delete this memory?')) return
    try {
        const res = await fetch(`/api/memories/${id}`, { method: 'DELETE' })
        if (res.ok) {
            fetchMemories()
        }
    } catch (e) {
        alert('Failed to delete')
    }
}

const logout = () => {
    localStorage.removeItem('user')
    router.push('/login')
}

onMounted(fetchMemories)
</script>

<template>
  <div class="dashboard">
    <div class="header flex justify-between items-center">
      <h1>Memory Dashboard</h1>
      <div class="flex items-center gap-4">
        <span>Welcome, {{ user.username }}</span>
        <button @click="logout" style="background-color: var(--surface-color); border: 1px solid var(--border-color)">Logout</button>
      </div>
    </div>

    <div class="memories-list">
      <div v-if="memories.length === 0" class="text-muted text-center" style="margin-top: 2rem;">
        No memories found.
      </div>
      
      <div v-for="mem in memories" :key="mem.id" class="card">
        <div class="flex justify-between">
          <span class="badge" :class="mem.type">{{ mem.type }}</span>
          <span class="text-sm text-muted">{{ new Date(mem.timestamp).toLocaleString() }}</span>
        </div>
        <p class="content" style="white-space: pre-wrap; margin: 1rem 0;">{{ mem.content }}</p>
        
        <div class="actions flex justify-end">
           <button @click="deleteMemory(mem.id)" style="background-color: var(--error-color); font-size: 0.8rem; padding: 0.5rem 1rem;">Delete</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.badge {
    text-transform: uppercase;
    font-size: 0.75rem;
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    font-weight: bold;
    background: var(--primary-color);
}
.badge.short_term { background: #8b5cf6; }
.badge.long_term { background: #10b981; }
.badge.entity { background: #f59e0b; }
</style>
