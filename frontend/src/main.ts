import { createApp } from 'vue'
import { createPinia } from 'pinia'
import NutUI from '@nutui/nutui'
import '@nutui/nutui/dist/style.css'
import '@nutui/icons-vue/dist/style_icon.css'
import App from './App.vue'
import router from './router'
import './styles.css'

createApp(App).use(createPinia()).use(router).use(NutUI).mount('#app')
