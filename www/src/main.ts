import { createApp } from "vue";
import App from "./App.vue";
import { router } from "./router/routes";
import { globals } from "./globals";
import { loadConstants } from "./constants";
import "./nativeIn"; // nativeIn must be imported from *somewhere*, to make sure that we expose our interface to our host WebView on mobile

loadConstants();

const app = createApp(App);
app.use(router);
app.directive("focus", {
	mounted: (el) => el.focus(),
});
app.mount("#app");

globals.bootup(true);
