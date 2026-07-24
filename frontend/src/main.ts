import { createApp } from 'vue'
import { createPinia } from 'pinia'
import NutUI from '@nutui/nutui'
import '@nutui/nutui/dist/style.css'
import '@nutui/icons-vue/dist/style_icon.css'
import App from './App.vue'
import router from './router'
import './styles.css'

const darkScheme = window.matchMedia('(prefers-color-scheme: dark)')
const syncNutTheme = () => document.documentElement.classList.toggle('nut-theme-dark', darkScheme.matches)
syncNutTheme()
darkScheme.addEventListener('change', syncNutTheme)

createApp(App).use(createPinia()).use(router).use(NutUI).mount('#app')
