import { createRouter, createWebHashHistory } from 'vue-router'
import Login from './views/Login.vue'
import AdminLayout from './layouts/AdminLayout.vue'
import MemoryExplorer from './views/MemoryExplorer.vue'
import Users from './views/Users.vue'
import Status from './views/Status.vue'
import StagingReview from './views/StagingReview.vue'
import MonitoringDashboard from './views/MonitoringDashboard.vue'
import AdminControl from './views/AdminControl.vue'

const routes = [
    { path: '/login', component: Login, meta: { title: 'AI Memory Admin Login' } },
    {
        path: '/admin',
        component: AdminLayout,
        meta: { requiresAuth: true },
        children: [
            { path: 'memory', component: MemoryExplorer },
            { path: 'staging', component: StagingReview },
            { path: 'monitoring', component: MonitoringDashboard },
            { path: 'alerts', component: () => import('./views/AlertCenter.vue'), meta: { title: 'Alert Center' } },
            { path: 'control', component: AdminControl },
            { path: 'users', component: Users },
            { path: 'status', component: Status },
            { path: '', redirect: 'memory' }
        ]
    },
    { path: '/', redirect: '/login' },
    { path: '/dashboard', redirect: '/admin' }
]

const router = createRouter({
    history: createWebHashHistory(),
    routes
})

router.beforeEach((to, from, next) => {
    const isAuthenticated = !!localStorage.getItem('user')
    if (to.meta.requiresAuth && !isAuthenticated) {
        next('/login')
    } else {
        next()
    }
})

export default router
