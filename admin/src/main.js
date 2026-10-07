import { createApp } from 'vue'
import App from './App.vue'
import i18n, { currentLocale } from './i18n'
import router from './router'
import './styles/base.css'

document.documentElement.setAttribute('lang', currentLocale())
createApp(App).use(router).use(i18n).mount('#app')
