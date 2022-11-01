import { createApp } from "vue";
import App from "./App.vue";
import { router } from "./router/routes";
import { globals } from "./globals";
import { loadConstants } from "./constants";
import "./natcom";

loadConstants();

const app = createApp(App);
app.use(router);
app.directive("focus", {
	mounted: (el) => el.focus(),
});
app.mount("#app");

globals.loadSystemInfo();
