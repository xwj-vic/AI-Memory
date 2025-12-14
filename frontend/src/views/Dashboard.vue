<script setup>
import { ref, onMounted, reactive, watch } from 'vue'
import { useRouter } from 'vue-router'

const memories = ref([])
const router = useRouter()
const user = JSON.parse(localStorage.getItem('user') || '{}')

const filters = reactive({
    userId: '',
    type: 'all'
})
const page = ref(1)
const limit = ref(50)
const selectedMemory = ref(null)

const fetchMemories = async () => {
    try {
        const query = new URLSearchParams({
            user_id: filters.userId,
            type: filters.type,
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

const search = () => {
    page.value = 1
    fetchMemories()
}

const prevPage = () => {
    if (page.value > 1) {
        page.value--
        fetchMemories()
    }
}

const nextPage = () => {
    page.value++
    fetchMemories() // Ideally check if has more, but basic impl for now
}

const isEditing = ref(false)
const editContent = ref('')

const openModal = (mem) => {
    selectedMemory.value = mem
    isEditing.value = false
    editContent.value = mem.content
}

const closeModal = () => {
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
            alert('Updated!')
            selectedMemory.value.content = editContent.value
            isEditing.value = false
            fetchMemories()
        } else {
            alert('Failed to update')
        }
    } catch (e) {
        console.error(e)
        alert('Error updating')
    }
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

    <!-- Filters -->
    <div class="filters flex gap-4 items-center" style="margin-bottom: 1rem; padding: 1rem; background: var(--surface-color); border-radius: 8px;">
        <input v-model="filters.userId" placeholder="Filter by User ID" style="padding: 0.5rem; border-radius: 4px; border: 1px solid var(--border-color); color: var(--text-color); background: var(--background-color);">
        <select v-model="filters.type" style="padding: 0.5rem; border-radius: 4px; border: 1px solid var(--border-color); color: var(--text-color); background: var(--background-color);">
            <option value="all">All Types</option>
            <option value="short_term">Short Term</option>
            <option value="long_term">Long Term</option>
        </select>
        <button @click="search" style="padding: 0.5rem 1rem;">Search</button>
    </div>

    <div class="memories-list">
      <div v-if="memories.length === 0" class="text-muted text-center" style="margin-top: 2rem;">
        No memories found.
      </div>
      
      <div v-for="mem in memories" :key="mem.id" class="card" @click="openModal(mem)" style="cursor: pointer;">
        <div class="flex justify-between">
          <span class="badge" :class="mem.type">{{ mem.type }}</span>
          <span class="text-sm text-muted">{{ new Date(mem.timestamp).toLocaleString() }}</span>
        </div>
        <p class="content" style="white-space: pre-wrap; margin: 1rem 0; max-height: 100px; overflow: hidden; text-overflow: ellipsis; display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical;">{{ mem.content }}</p>
        
        <div class="meta text-sm text-muted" v-if="mem.metadata && mem.metadata.user_id">
            User: {{ mem.metadata.user_id }}
        </div>

        <div class="actions flex justify-end">
           <button @click.stop="deleteMemory(mem.id)" style="background-color: var(--error-color); font-size: 0.8rem; padding: 0.5rem 1rem;">Delete</button>
        </div>
      </div>
    </div>

    <!-- Pagination -->
    <div class="pagination flex justify-center gap-4 items-center" style="margin-top: 2rem;" v-if="memories.length > 0 || page > 1">
        <button @click="prevPage" :disabled="page <= 1">Previous</button>
        <span>Page {{ page }}</span>
        <button @click="nextPage">Next</button>
    </div>

    <!-- Modal -->
    <div v-if="selectedMemory" class="modal-overlay" @click.self="closeModal">
        <div class="modal-content">
            <div class="modal-header">
                <h2>Memory Details</h2>
                <button class="close-btn" @click="closeModal">&times;</button>
            </div>
            
            <div class="modal-body">
                <div class="info-row">
                    <span class="id-tag">{{ selectedMemory.id }}</span>
                    <span class="badge" :class="selectedMemory.type">{{ selectedMemory.type }}</span>
                    <span class="timestamp">{{ new Date(selectedMemory.timestamp).toLocaleString() }}</span>
                </div>

                <div class="form-group">
                    <label>Content</label>
                    <textarea v-if="isEditing" v-model="editContent" class="edit-area"></textarea>
                    <div v-else class="content-view">{{ selectedMemory.content }}</div>
                </div>

                <div class="form-group">
                    <label>Metadata</label>
                    <pre class="metadata-view">{{ JSON.stringify(selectedMemory.metadata, null, 2) }}</pre>
                </div>
            </div>

            <div class="modal-footer">
                <button v-if="!isEditing" @click="isEditing = true" class="btn btn-primary">Edit</button>
                <button v-if="isEditing" @click="updateMemory" class="btn btn-success">Update</button>
                <button @click="closeModal" class="btn btn-secondary">Close</button>
            </div>
        </div>
    </div>
  </div>
</template>

<style scoped>
.dashboard {
    max-width: 1200px;
    margin: 0 auto;
    padding: 2rem;
}

.header {
    margin-bottom: 2rem;
}

h1 {
    font-size: 1.5rem;
    font-weight: bold;
}

.filters {
    display: flex;
    gap: 1rem;
    align-items: center;
    margin-bottom: 1rem;
    padding: 1rem;
    background: var(--surface-color);
    border-radius: 8px;
    border: 1px solid var(--border-color);
}

.card {
    background: var(--surface-color);
    border: 1px solid var(--border-color);
    padding: 1rem;
    border-radius: 8px;
    margin-bottom: 1rem;
    cursor: pointer;
    transition: transform 0.1s;
}
.card:hover {
    transform: translateY(-2px);
}

.badge {
    text-transform: uppercase;
    font-size: 0.75rem;
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    font-weight: bold;
    color: white;
}
.badge.short_term { background: #8b5cf6; }
.badge.long_term { background: #10b981; }

.modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 1000;
}

.modal-content {
    background: var(--surface-color);
    border-radius: 12px;
    width: 90%;
    max-width: 600px;
    max-height: 85vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 10px 25px rgba(0,0,0,0.2);
    border: 1px solid var(--border-color);
    overflow: hidden;
}

.modal-header {
    padding: 1rem 1.5rem;
    border-bottom: 1px solid var(--border-color);
    display: flex;
    justify-content: space-between;
    align-items: center;
}
.modal-header h2 { margin: 0; font-size: 1.25rem; }

.close-btn {
    background: none;
    border: none;
    font-size: 1.5rem;
    color: var(--text-color);
    opacity: 0.6;
    cursor: pointer;
}
.close-btn:hover { opacity: 1; }

.modal-body {
    padding: 1.5rem;
    overflow-y: auto;
    flex: 1;
}

.info-row {
    display: flex;
    gap: 0.8rem;
    align-items: center;
    margin-bottom: 1.5rem;
    flex-wrap: wrap;
}

.id-tag {
    font-family: monospace;
    background: rgba(0,0,0,0.1);
    padding: 2px 6px;
    border-radius: 4px;
    font-size: 0.8rem;
}

.timestamp {
    font-size: 0.85rem;
    color: #888;
}

.form-group {
    margin-bottom: 1.5rem;
}
.form-group label {
    display: block;
    margin-bottom: 0.5rem;
    font-weight: 500;
    font-size: 0.9rem;
    color: #666;
}

.edit-area, .content-view, .metadata-view {
    width: 100%;
    padding: 0.75rem;
    border-radius: 6px;
    border: 1px solid var(--border-color);
    background: rgba(0,0,0,0.02);
    font-size: 0.95rem;
    color: var(--text-color);
}

.edit-area {
    min-height: 150px;
    resize: vertical;
}

.content-view {
    white-space: pre-wrap;
    max-height: 300px;
    overflow-y: auto;
}

.metadata-view {
    font-family: monospace;
    font-size: 0.85rem;
    overflow-x: auto;
    background: #0000000d;
}

.modal-footer {
    padding: 1rem 1.5rem;
    border-top: 1px solid var(--border-color);
    display: flex;
    justify-content: flex-end;
    gap: 0.75rem;
    background: rgba(0,0,0,0.02);
}

.btn {
    padding: 0.5rem 1rem;
    border-radius: 6px;
    border: none;
    cursor: pointer;
    font-weight: 500;
    transition: background 0.15s;
}

.btn-primary { background: #3b82f6; color: white; }
.btn-primary:hover { background: #2563eb; }

.btn-success { background: #10b981; color: white; }
.btn-success:hover { background: #059669; }

.btn-secondary { background: #e5e7eb; color: #374151; }
.btn-secondary:hover { background: #d1d5db; }
</style>
