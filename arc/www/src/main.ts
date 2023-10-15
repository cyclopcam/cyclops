import './assets/main.scss'

import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import { globals } from './globals'

async function boot() {
	if (window.location.pathname !== "/login") {
		// The constants that we fetch here affect our HTML, for example the publicVideoBaseUrl.
		// This is why we need to fetch them before creating our app.
		// We should really have some kind of welcome spinner.
		await globals.fetchConstantsAndRedirectIfNotLoggedIn();
	}
	const app = createApp(App);
	app.use(router);
	app.mount('#app');
}

boot();