import { createApp } from "vue";
import App from "./App.vue";
import { router, replaceRoute } from "./router/routes";
import { debugMode } from "./constants";
import "./assets/base.scss";

let app = createApp(App);
app.use(router);
app.directive("focus", {
	mounted: (el) => el.focus(),
});
app.mount("#app");

// debug
if (debugMode) {
	replaceRoute({ name: 'rtDefault' });
}
