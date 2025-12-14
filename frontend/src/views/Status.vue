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

onMounted(fetchStatus)
</script>

<template>
<div>
    <h2 class="text-2xl font-bold mb-6">System Status</h2>
    
    <div class="grid grid-cols-2 gap-6">
        <div class="bg-white p-6 rounded-lg shadow border" v-for="(val, key) in status" :key="key">
            <h3 class="text-gray-500 text-sm font-medium uppercase mb-2">{{ key }}</h3>
            <div class="text-3xl font-bold" :class="{'text-green-600': val === 'Online', 'text-red-600': val !== 'Online'}">
                {{ val }}
            </div>
        </div>
    </div>
</div>
</template>

<style scoped>
/* CSS */
.text-2xl { font-size: 1.5rem; }
.font-bold { font-weight: 700; }
.mb-6 { margin-bottom: 1.5rem; }
.grid { display: grid; }
.grid-cols-2 { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.gap-6 { gap: 1.5rem; }
.bg-white { background-color: white; }
.p-6 { padding: 1.5rem; }
.rounded-lg { border-radius: 0.5rem; }
.shadow { box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
.border { border: 1px solid #e5e7eb; }
.text-gray-500 { color: #6b7280; }
.text-sm { font-size: 0.875rem; }
.font-medium { font-weight: 500; }
.uppercase { text-transform: uppercase; }
.mb-2 { margin-bottom: 0.5rem; }
.text-3xl { font-size: 1.875rem; line-height: 2.25rem; }
.text-green-600 { color: #059669; }
.text-red-600 { color: #dc2626; }
</style>
