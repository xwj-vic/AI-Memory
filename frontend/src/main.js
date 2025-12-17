import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'

import './style.css'
import './styles/element-custom.css'
import App from './App.vue'
import router from './router'
import i18n from './i18n'

const app = createApp(App)

// 注册所有Element Plus图标
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
    app.component(key, component)
}

// 根据localStorage动态设置Element Plus语言
const savedLocale = localStorage.getItem('locale') || 'zh'
const elementLocale = savedLocale === 'zh' ? zhCn : en

app.use(ElementPlus, {
    locale: elementLocale
})
app.use(router)
app.use(i18n)
app.mount('#app')
