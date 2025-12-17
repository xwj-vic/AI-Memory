<template>
  <el-dropdown @command="handleCommand" trigger="click">
    <el-button>
      {{ currentLang }}
      <el-icon class="el-icon--right"><ArrowDown /></el-icon>
    </el-button>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item command="zh" :disabled="locale === 'zh'">
          🇨🇳 简体中文
        </el-dropdown-item>
        <el-dropdown-item command="en" :disabled="locale === 'en'">
          🇺🇸 English
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'
import ElementPlus from 'element-plus'

const { locale } = useI18n()

const currentLang = computed(() => {
  return locale.value === 'zh' ? '中文' : 'EN'
})

const handleCommand = (lang) => {
  locale.value = lang
  localStorage.setItem('locale', lang)
  
  // 重新加载页面以应用Element Plus语言
  window.location.reload()
}
</script>
