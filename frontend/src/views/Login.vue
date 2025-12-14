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
    router.push('/dashboard')
  } catch (e) {
    error.value = 'Invalid credentials'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-container flex items-center justify-center">
    <div class="card w-full" style="max-width: 400px;">
      <h1>Admin Login</h1>
      
      <form @submit.prevent="handleLogin" class="flex flex-col gap-4">
        <div>
          <label>Username</label>
          <input v-model="username" type="text" required placeholder="admin" />
        </div>
        
        <div>
          <label>Password</label>
          <input v-model="password" type="password" required placeholder="••••••" />
        </div>

        <div v-if="error" class="error-msg text-sm" style="color: var(--error-color)">
          {{ error }}
        </div>

        <button type="submit" :disabled="loading">
          {{ loading ? 'Logging in...' : 'Login' }}
        </button>
      </form>
    </div>
  </div>
</template>

<style scoped>
.login-container {
  min-height: 80vh;
}
</style>
