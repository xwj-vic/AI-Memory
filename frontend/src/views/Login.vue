<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { User, Lock } from '@element-plus/icons-vue'

const router = useRouter()
const loading = ref(false)
const loginForm = reactive({
  username: '',
  password: ''
})

const handleLogin = async () => {
  loading.value = true
  
  try {
    const res = await fetch('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(loginForm)
    })
    
    if (!res.ok) {
      throw new Error('Login failed')
    }
    
    const data = await res.json()
    localStorage.setItem('user', JSON.stringify(data.user))
    
    ElMessage.success('登录成功！')
    router.push('/admin/memory')
  } catch (e) {
    ElMessage.error('用户名或密码错误')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-container">
    <div class="login-background">
      <div class="gradient-orb orb-1"></div>
      <div class="gradient-orb orb-2"></div>
      <div class="gradient-orb orb-3"></div>
    </div>
    
    <el-card class="login-card" shadow="always">
      <div class="login-header">
        <div class="logo-container">
          <div class="logo">🧠</div>
        </div>
        <h1 class="title">{{ $t('login.title') }}</h1>
        <p class="subtitle">{{ $t('login.subtitle') }}</p>
      </div>
      
      <el-form 
        :model="loginForm" 
        @submit.prevent="handleLogin"
        class="login-form"
        size="large"
      >
        <el-form-item>
          <el-input
            v-model="loginForm.username"
            :placeholder="$t('login.username')"
            size="large"
            clearable
          />
        </el-form-item>
        
        <el-form-item>
          <el-input
            v-model="loginForm.password"
            type="password"
            :placeholder="$t('login.password')"
            size="large"
            @keyup.enter="handleLogin"
          />
        </el-form-item>
        
        <el-button 
          type="primary" 
          size="large"
          :loading="loading"
          @click="handleLogin"
          class="login-btn"
        >
          {{ loading ? $t('common.loading') : $t('login.login') }}
        </el-button>
      </el-form>
      
      <div class="login-footer">
        <span>AI-Memory v1.0</span>
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  overflow: hidden;
  padding: 20px;
}

.login-background {
  position: absolute;
  inset: 0;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  z-index: 0;
}

.gradient-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.6;
  animation: float 20s infinite ease-in-out;
}

.orb-1 {
  width: 400px;
  height: 400px;
  background: #ff6b6b;
  top: -100px;
  left: -100px;
  animation-delay: 0s;
}

.orb-2 {
  width: 300px;
  height: 300px;
  background: #4ecdc4;
  bottom: -80px;
  right: -80px;
  animation-delay: 7s;
}

.orb-3 {
  width: 250px;
  height: 250px;
  background: #ffe66d;
  top: 50%;
  right: 20%;
  animation-delay: 14s;
}

@keyframes float {
  0%, 100% {
    transform: translate(0, 0) scale(1);
  }
  33% {
    transform: translate(50px, -50px) scale(1.1);
  }
  66% {
    transform: translate(-50px, 50px) scale(0.9);
  }
}

.login-card {
  width: 100%;
  max-width: 420px;
  padding: 20px;
  backdrop-filter: blur(20px);
  background: rgba(255, 255, 255, 0.95) !important;
  border: 1px solid rgba(255, 255, 255, 0.3);
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  position: relative;
  z-index: 1;
}

.login-header {
  text-align: center;
  margin-bottom: 32px;
}

.logo-wrapper {
  display: flex;
  justify-content: center;
  margin-bottom: 16px;
}

.logo-icon {
  width: 80px;
  height: 80px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 48px;
  background: linear-gradient(135deg, #667eea, #764ba2);
  border-radius: 20px;
  box-shadow: 0 8px 24px rgba(118, 75, 162, 0.4);
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0%, 100% {
    transform: scale(1);
    box-shadow: 0 8px 24px rgba(118, 75, 162, 0.4);
  }
  50% {
    transform: scale(1.05);
    box-shadow: 0 12px 32px rgba(118, 75, 162, 0.6);
  }
}

.login-header h1 {
  font-size: 28px;
  font-weight: 700;
  background: linear-gradient(135deg, #667eea, #764ba2);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  margin: 0 0 8px 0;
}

.login-header p {
  color: #666;
  font-size: 14px;
  margin: 0;
}

.login-form {
  margin-bottom: 24px;
}

.login-button {
  width: 100%;
  height: 44px;
  font-size: 16px;
  font-weight: 600;
  background: linear-gradient(135deg, #667eea, #764ba2);
  border: none;
  box-shadow: 0 4px 15px rgba(102, 126, 234, 0.4);
  transition: all 0.3s;
}

.login-button:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(102, 126, 234, 0.6);
}

.login-button:active {
  transform: translateY(0);
}

.login-footer {
  text-align: center;
  color: #999;
  font-size: 12px;
  padding-top: 16px;
  border-top: 1px solid #eee;
}

/* 响应式 */
@media (max-width: 768px) {
  .login-card {
    padding: 24px 16px;
  }
  
  .logo-icon {
    width: 60px;
    height: 60px;
    font-size: 36px;
  }
  
  .login-header h1 {
    font-size: 24px;
  }
}
</style>
