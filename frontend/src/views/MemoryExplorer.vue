<script setup>
import { ref, onMounted, reactive, watch, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'

const memories = ref([])
const router = useRouter()
const route = useRoute()
const user = JSON.parse(localStorage.getItem('user') || '{}')

const activeTab = ref('long_term') // 'long_term' or 'short_term'
const filters = reactive({
    userId: '',
})
const page = ref(1)
const limit = ref(50)
const selectedMemory = ref(null)

// Initialize from Route
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
            type: activeTab.value, // Fetch based on active tab
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

// Watchers
watch(activeTab, () => {
    page.value = 1
    fetchMemories()
})

watch(() => filters.userId, () => {
    // Optional: Debounce could be good here
})

// Grouped Short Term Memories
const groupedShortTerm = computed(() => {
    if (activeTab.value !== 'short_term') return {}
    
    // Group by session_id
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
    fetchMemories()
}

// Modal Logic
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
</script>

<template>
  <div class="explorer-page">
    <header class="page-header flex-between">
      <div class="header-title">
        <h1>Memory Explorer</h1>
        <p class="text-muted text-sm">Manage AI long-term and short-term knowledge.</p>
      </div>
      <div class="user-badge text-sm">
        <span class="text-muted">Admin:</span> <strong>{{ user.username }}</strong>
      </div>
    </header>

    <!-- Controls -->
    <div class="card controls-bar flex-between">
        <div class="search-box">
            <input v-model="filters.userId" placeholder="Filter by User Identifier..." class="form-input">
        </div>
        <button @click="search" class="btn btn-primary">Search</button>
    </div>

    <!-- Tabs -->
    <div class="tabs">
        <button 
            @click="activeTab = 'long_term'"
            class="tab-btn"
            :class="{ active: activeTab === 'long_term' }"
        >
            Long Term Memory
        </button>
        <button 
            @click="activeTab = 'short_term'"
            class="tab-btn"
            :class="{ active: activeTab === 'short_term' }"
        >
            Short Term Memory
        </button>
    </div>

    <!-- Content Area -->
    <div class="memories-container">
        
        <!-- Loading / Empty State -->
        <div v-if="memories.length === 0" class="empty-state">
            <p>No memories found for this criteria.</p>
        </div>

        <!-- Long Term View -->
        <div v-if="activeTab === 'long_term' && memories.length > 0" class="memory-grid">
             <div v-for="mem in memories" :key="mem.id" class="card memory-card" @click="openModal(mem)">
                <div class="card-header flex-between">
                    <span class="badge badge-purple">LTM</span>
                    <span class="timestamp">{{ new Date(mem.timestamp).toLocaleString() }}</span>
                </div>
                <div class="card-body">
                     <p class="memory-text">{{ mem.content }}</p>
                </div>
                <div class="card-footer flex-between">
                    <span class="user-id">User: {{ mem.metadata?.user_id || 'N/A' }}</span>
                     <button @click.stop="deleteMemory(mem.id)" class="btn btn-danger btn-icon" title="Delete">&times;</button>
                </div>
             </div>
        </div>

        <!-- Short Term View (Grouped) -->
        <div v-if="activeTab === 'short_term' && memories.length > 0" class="session-list">
            <div v-for="(sessionMemories, sessionId) in groupedShortTerm" :key="sessionId" class="session-group">
                <div class="session-header">
                    <span class="session-label">Session: {{ sessionId }}</span>
                    <div class="divider"></div>
                </div>

                <div class="session-memories">
                    <div v-for="mem in sessionMemories" :key="mem.id" class="session-item" @click="openModal(mem)">
                         <div class="session-meta flex-between">
                            <span class="timestamp">{{ new Date(mem.timestamp).toLocaleTimeString() }}</span>
                            <button @click.stop="deleteMemory(mem.id)" class="delete-link">&times;</button>
                        </div>
                        <p class="session-text">{{ mem.content }}</p>
                    </div>
                </div>
            </div>
        </div>

        <!-- Pagination -->
        <div class="pagination flex-center" v-if="memories.length > 0">
             <button @click="prevPage" :disabled="page <= 1" class="btn btn-ghost">Previous</button>
             <span class="page-info">Page {{ page }}</span>
             <button @click="nextPage" class="btn btn-ghost">Next</button>
        </div>
    </div>

    <!-- Modal -->
    <div v-if="selectedMemory" class="modal-backdrop" @click.self="closeModal">
        <div class="modal card">
            <div class="modal-header flex-between">
                <h3>Memory Details</h3>
                <button @click="closeModal" class="close-btn">&times;</button>
            </div>
            
            <div class="modal-body">
                <div class="meta-row">
                    <span class="badge badge-gray">{{ selectedMemory.id }}</span>
                    <span class="badge badge-blue">{{ selectedMemory.type }}</span>
                    <span class="timestamp">{{ new Date(selectedMemory.timestamp).toLocaleString() }}</span>
                </div>

                <div class="form-group">
                    <label class="form-label">Content</label>
                    <textarea v-if="isEditing" v-model="editContent" class="form-textarea" rows="8"></textarea>
                    <div v-else class="content-view">{{ selectedMemory.content }}</div>
                </div>

                <div class="form-group">
                     <label class="form-label">Metadata</label>
                     <pre class="code-block">{{ JSON.stringify(selectedMemory.metadata, null, 2) }}</pre>
                </div>
            </div>

            <div class="modal-footer">
                <button v-if="!isEditing" @click="isEditing = true" class="btn btn-primary">Edit</button>
                <button v-if="isEditing" @click="updateMemory" class="btn btn-primary">Save Changes</button>
                <button @click="closeModal" class="btn btn-ghost">Close</button>
            </div>
        </div>
    </div>

  </div>
</template>

<style scoped>
.page-header {
    margin-bottom: 2rem;
}

.controls-bar {
    padding: 1rem;
    margin-bottom: 2rem;
    gap: 1rem;
}

.search-box {
    flex: 1;
}

/* Tabs */
.tabs {
    display: flex;
    gap: 2rem;
    border-bottom: 2px solid var(--color-surface-200);
    margin-bottom: 2rem;
}

.tab-btn {
    background: none;
    border: none;
    padding: 1rem 0;
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--color-text-muted);
    cursor: pointer;
    position: relative;
    top: 2px;
    border-bottom: 2px solid transparent;
    transition: all var(--transition-fast);
}

.tab-btn:hover {
    color: var(--color-surface-900);
}

.tab-btn.active {
    color: var(--color-primary-600);
    border-bottom: 2px solid var(--color-primary-600);
}

/* Memory Grid */
.memory-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 1.5rem;
}

.memory-card {
    padding: 1.25rem;
    cursor: pointer;
    transition: box-shadow var(--transition-fast), transform var(--transition-fast);
    display: flex;
    flex-direction: column;
    height: 200px;
}

.memory-card:hover {
    box-shadow: var(--shadow-md);
    transform: translateY(-2px);
    border-color: var(--color-primary-200);
}

.card-header {
    margin-bottom: 1rem;
}

.memory-text {
    font-size: 0.875rem;
    color: var(--color-surface-800);
    display: -webkit-box;
    -webkit-line-clamp: 4;
    -webkit-box-orient: vertical;
    overflow: hidden;
    margin: 0;
    line-height: 1.6;
}

.card-body {
    flex: 1;
    overflow: hidden;
    margin-bottom: 1rem;
}

.card-footer {
    font-size: 0.75rem;
    color: var(--color-text-muted);
}

/* Session List */
.session-group {
    margin-bottom: 2rem;
}

.session-header {
    display: flex;
    align-items: center;
    gap: 1rem;
    margin-bottom: 1rem;
}

.session-label {
    font-family: var(--font-mono);
    font-size: 0.75rem;
    text-transform: uppercase;
    background-color: var(--color-surface-200);
    padding: 0.25rem 0.5rem;
    border-radius: var(--radius-sm);
    color: var(--color-surface-800);
}

.divider {
    flex: 1;
    height: 1px;
    background-color: var(--color-surface-200);
}

.session-memories {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    padding-left: 1rem;
    border-left: 2px solid var(--color-surface-100);
}

.session-item {
    background: var(--color-base-white);
    padding: 1rem;
    border-radius: var(--radius-md);
    border: 1px solid var(--color-surface-200);
    cursor: pointer;
    transition: border-color var(--transition-fast);
}

.session-item:hover {
    border-color: var(--color-primary-300);
}

.session-meta {
    margin-bottom: 0.5rem;
}

.session-text {
    font-size: 0.875rem;
    margin: 0;
    color: var(--color-surface-800);
}

.delete-link {
    background: none;
    border: none;
    color: var(--color-text-muted);
    font-size: 1.25rem;
    cursor: pointer;
    line-height: 1;
    padding: 0 0.5rem;
}

.delete-link:hover {
    color: var(--color-danger);
}

/* Utils */
.badge {
    padding: 0.25rem 0.5rem;
    border-radius: var(--radius-sm);
    font-size: 0.75rem;
    font-weight: 600;
}

.badge-purple {
    background-color: var(--color-primary-100);
    color: var(--color-primary-700);
}

.badge-blue {
    background-color: #e0f2fe;
    color: #0284c7;
}

.badge-gray {
    background-color: var(--color-surface-100);
    font-family: var(--font-mono);
}

.timestamp {
    color: var(--color-text-muted);
    font-size: 0.75rem;
}

.empty-state {
    text-align: center;
    padding: 4rem;
    color: var(--color-text-muted);
    background-color: var(--color-surface-50);
    border-radius: var(--radius-lg);
    border: 1px dashed var(--color-surface-300);
}

.pagination {
    margin-top: 2rem;
    gap: 1rem;
}

.page-info {
    font-size: 0.875rem;
    color: var(--color-text-muted);
}

/* Modal */
.modal-backdrop {
    position: fixed;
    inset: 0;
    background-color: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 50;
    backdrop-filter: blur(4px);
    padding: 1rem;
}

.modal {
    width: 600px;
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    animation: modalSlide 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

@keyframes modalSlide {
    from { opacity: 0; transform: translateY(20px); }
    to { opacity: 1; transform: translateY(0); }
}

.modal-header {
    padding: 1.25rem;
    border-bottom: 1px solid var(--color-surface-200);
}

.modal-body {
    padding: 1.5rem;
    overflow-y: auto;
    flex: 1;
}

.modal-footer {
    padding: 1.25rem;
    border-top: 1px solid var(--color-surface-200);
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    background-color: var(--color-surface-50);
    border-radius: 0 0 var(--radius-lg) var(--radius-lg);
}

.close-btn {
    background: none;
    border: none;
    font-size: 1.5rem;
    cursor: pointer;
    color: var(--color-text-muted);
    line-height: 1;
}

.meta-row {
    display: flex;
    gap: 0.75rem;
    align-items: center;
    margin-bottom: 1.5rem;
}

.content-view {
    padding: 1rem;
    background-color: var(--color-surface-50);
    border: 1px solid var(--color-surface-200);
    border-radius: var(--radius-md);
    white-space: pre-wrap;
    font-size: 0.875rem;
    line-height: 1.6;
}

.code-block {
    background-color: var(--color-surface-900);
    color: var(--color-surface-50);
    padding: 1rem;
    border-radius: var(--radius-md);
    font-size: 0.75rem;
    overflow-x: auto;
    margin: 0;
}
</style>
