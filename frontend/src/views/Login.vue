<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)
const router = useRouter()

const handleLogin = async () => {
  loading.value = true
  error.value = ''
  
  try {
    const res = await fetch('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: username.value, password: password.value })
    })
    
    if (!res.ok) {
        throw new Error('Login failed')
    }
    
    const data = await res.json()
    // Store user data
    localStorage.setItem('user', JSON.stringify(data.user))
    router.push('/admin/memory')
  } catch (e) {
    error.value = 'Invalid credentials'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-wrapper">
    <div class="glass-card login-card">
        <div class="login-header">
            <h1>AI Memory</h1>
            <p class="text-muted">Admin Dashboard Access</p>
        </div>
      
      <form @submit.prevent="handleLogin" class="login-form">
        <div class="form-group">
          <label class="form-label">Username</label>
          <input v-model="username" type="text" class="form-input" required placeholder="Enter username" />
        </div>
        
        <div class="form-group">
          <label class="form-label">Password</label>
          <input v-model="password" type="password" class="form-input" required placeholder="••••••" />
        </div>

        <div v-if="error" class="error-msg text-sm">
          {{ error }}
        </div>

        <button type="submit" class="btn btn-primary w-full" :disabled="loading">
          {{ loading ? 'Authenticating...' : 'Sign In' }}
        </button>
      </form>
    </div>
  </div>
</template>

<style scoped>
.login-wrapper {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: radial-gradient(circle at top right, var(--color-primary-100), transparent 40%),
              radial-gradient(circle at bottom left, var(--color-surface-200), transparent 40%);
  padding: 1rem;
}

.login-card {
  width: 100%;
  max-width: 420px;
  padding: 2.5rem;
}

.login-header {
  text-align: center;
  margin-bottom: 2rem;
}

.login-header h1 {
  font-size: 2rem;
  background: linear-gradient(to right, var(--color-primary-600), #8b5cf6);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
  margin-bottom: 0.5rem;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.error-msg {
  color: var(--color-danger);
  background-color: var(--color-danger-bg);
  padding: 0.75rem;
  border-radius: var(--radius-md);
  margin-bottom: 1rem;
  text-align: center;
}
</style>
