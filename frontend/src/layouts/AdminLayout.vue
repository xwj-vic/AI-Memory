<script setup>
import { useRouter } from 'vue-router'
import LanguageSwitcher from '../components/LanguageSwitcher.vue'

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
            <router-link to="/admin/memory" class="nav-item">📚 {{ $t('nav.memory') }}</router-link>
            <router-link to="/admin/staging" class="nav-item">🔍 {{ $t('nav.staging') }}</router-link>
            <router-link to="/admin/monitoring" class="nav-item">📊 {{ $t('nav.monitoring') }}</router-link>
            <router-link to="/admin/alerts" class="nav-item">🔔 {{ $t('nav.alerts') }}</router-link>
            <router-link to="/admin/control" class="nav-item">🎛️ {{ $t('nav.control') }}</router-link>
            <router-link to="/admin/users" class="nav-item">👥 {{ $t('nav.users') }}</router-link>
            <router-link to="/admin/status" class="nav-item">⚡ {{ $t('nav.status') }}</router-link>
        </nav>
        <div class="sidebar-footer">
            <div class="user-info">{{ $t('common.loggedInAs') }} {{ user.username }}</div>
            <div style="display: flex; justify-content: center; margin-bottom: 12px;">
              <LanguageSwitcher />
            </div>
            <button @click="logout" class="btn btn-ghost w-full">{{ $t('common.logout') }}</button>
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
