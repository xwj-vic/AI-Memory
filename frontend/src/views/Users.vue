<script setup>
import { ref, onMounted } from 'vue'

const users = ref([])
const loading = ref(true)

const fetchUsers = async () => {
    try {
        const res = await fetch('/api/users')
        if (res.ok) {
            const data = await res.json()
            users.value = data.users || []
        }
    } catch (e) {
        console.error(e)
    } finally {
        loading.value = false
    }
}

const formatDate = (dateStr) => {
    if (!dateStr) return 'Never'
    return new Date(dateStr).toLocaleString()
}

onMounted(fetchUsers)
</script>

<template>
<div>
    <h2 class="text-2xl font-bold mb-6">User Management</h2>
    
    <div class="bg-white rounded-lg shadow border overflow-hidden">
        <table class="min-w-full divide-y divide-gray-200">
            <thead class="bg-gray-50">
                <tr>
                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">User Identifier</th>
                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Last Active</th>
                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Sessions (STM)</th>
                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">LTM Items</th>
                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th>
                </tr>
            </thead>
            <tbody class="bg-white divide-y divide-gray-200">
                <tr v-if="loading">
                    <td colspan="5" class="px-6 py-4 text-center">Loading...</td>
                </tr>
                <tr v-else-if="users.length === 0">
                    <td colspan="5" class="px-6 py-4 text-center">No active users found.</td>
                </tr>
                <tr v-for="user in users" :key="user.id">
                    <td class="px-6 py-4 whitespace-nowrap font-mono text-sm font-medium text-gray-900">{{ user.user_identifier }}</td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {{ formatDate(user.last_active) }}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
                            {{ user.session_count }}
                        </span>
                    </td>
                     <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-purple-100 text-purple-800">
                            {{ user.ltm_count }}
                        </span>
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm font-medium">
                        <router-link :to="`/admin/memory?userId=${user.user_identifier}`" class="text-indigo-600 hover:text-indigo-900">View Memories</router-link>
                    </td>
                </tr>
            </tbody>
        </table>
    </div>
</div>
</template>

<style scoped>
/* Standard CSS */
.text-2xl { font-size: 1.5rem; line-height: 2rem; }
.font-bold { font-weight: 700; }
.mb-6 { margin-bottom: 1.5rem; }
.bg-white { background-color: white; }
.rounded-lg { border-radius: 0.5rem; }
.shadow { box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px 0 rgba(0, 0, 0, 0.06); }
.border { border: 1px solid #e5e7eb; }
.overflow-hidden { overflow: hidden; }
.min-w-full { min-width: 100%; }
.divide-y > * + * { border-top-width: 1px; border-color: #e5e7eb; }
.divide-gray-200 > * + * { border-color: #e5e7eb; }
.bg-gray-50 { background-color: #f9fafb; }
.px-6 { padding-left: 1.5rem; padding-right: 1.5rem; }
.py-3 { padding-top: 0.75rem; padding-bottom: 0.75rem; }
.text-left { text-align: left; }
.text-xs { font-size: 0.75rem; line-height: 1rem; }
.font-medium { font-weight: 500; }
.text-gray-500 { color: #6b7280; }
.text-gray-900 { color: #111827; }
.uppercase { text-transform: uppercase; }
.tracking-wider { letter-spacing: 0.05em; }
.bg-white { background-color: white; }
.px-4 { padding-left: 1rem; padding-right: 1rem; }
.text-center { text-align: center; }
.whitespace-nowrap { white-space: nowrap; }
.font-mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace; }
.text-sm { font-size: 0.875rem; line-height: 1.25rem; }
.inline-flex { display: inline-flex; }
.items-center { align-items: center; }
.px-2\.5 { padding-left: 0.625rem; padding-right: 0.625rem; }
.py-0\.5 { padding-top: 0.125rem; padding-bottom: 0.125rem; }
.rounded-full { border-radius: 9999px; }
.bg-blue-100 { background-color: #dbeafe; }
.text-blue-800 { color: #1e40af; }
.bg-purple-100 { background-color: #f3e8ff; }
.text-purple-800 { color: #6b21a8; }
.text-indigo-600 { color: #4f46e5; }
.hover\:text-indigo-900:hover { color: #312e81; }
</style>
