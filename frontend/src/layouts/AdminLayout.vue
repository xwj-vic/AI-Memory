<script setup>
import { useRouter } from 'vue-router'

const router = useRouter()
const user = JSON.parse(localStorage.getItem('user') || '{}')

const logout = () => {
    localStorage.removeItem('user')
    router.push('/login')
}
</script>

<template>
<div class="admin-layout">
    <!-- Sidebar -->
    <aside class="sidebar">
        <div class="sidebar-header">
            AI Memory
        </div>
        <nav class="sidebar-nav">
            <router-link to="/admin/memory" class="nav-item">Memory Explorer</router-link>
            <router-link to="/admin/users" class="nav-item">User Management</router-link>
            <router-link to="/admin/status" class="nav-item">System Status</router-link>
        </nav>
        <div class="sidebar-footer">
            <div class="user-info">Logged in as {{ user.username }}</div>
            <button @click="logout" class="btn btn-ghost w-full">Logout</button>
        </div>
    </aside>

    <!-- Main Content -->
    <main class="main-content">
        <router-view></router-view>
    </main>
</div>
</template>

<style scoped>
.admin-layout {
    display: flex;
    height: 100vh;
    background-color: var(--color-surface-50);
}

.sidebar {
    width: 280px;
    background-color: var(--color-base-white);
    border-right: 1px solid var(--color-surface-200);
    display: flex;
    flex-direction: column;
}

.sidebar-header {
    height: 4rem;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 700;
    font-size: 1.25rem;
    color: var(--color-primary-600);
    border-bottom: 1px solid var(--color-surface-200);
}

.sidebar-nav {
    flex: 1;
    padding: 1.5rem 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
}

.nav-item {
    display: block;
    padding: 0.75rem 1rem;
    border-radius: var(--radius-md);
    color: var(--color-surface-800);
    font-weight: 500;
    transition: all var(--transition-fast);
}

.nav-item:hover {
    background-color: var(--color-surface-100);
}

.nav-item.router-link-active {
    background-color: var(--color-primary-50);
    color: var(--color-primary-700);
}

.sidebar-footer {
    padding: 1.5rem;
    border-top: 1px solid var(--color-surface-200);
}

.user-info {
    font-size: 0.875rem;
    color: var(--color-text-muted);
    margin-bottom: 0.75rem;
    text-align: center;
}

.main-content {
    flex: 1;
    overflow-y: auto;
    padding: 2rem;
}
</style>
