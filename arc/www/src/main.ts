import './assets/main.scss'

import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import { globals } from './globals'

const app = createApp(App);
app.use(router);
app.mount('#app');

if (window.location.pathname !== "/login") {
	globals.redirectIfNotLoggedIn();
}


