import { createApp } from "vue";
import App from "./App.vue";
import { router, replaceRoute } from "./router/routes";
import { dummyMode } from "./constants";
import "./assets/base.scss";
import { globals } from "./global";

let app = createApp(App);
app.use(router);
app.directive("focus", {
	mounted: (el) => el.focus(),
});
app.mount("#app");

console.log("app has been mounted");

globals.startup();
