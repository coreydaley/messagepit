import App from "./App.vue";
import router from "./router";
import { createApp } from "vue";
import mitt from "mitt";

import "@fontsource/ibm-plex-sans/400.css";
import "@fontsource/ibm-plex-sans/500.css";
import "@fontsource/ibm-plex-sans/600.css";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/500.css";
import "./assets/styles.scss";
import "bootstrap-icons/font/bootstrap-icons.scss";
import "bootstrap";
import "vue-css-donut-chart/src/styles/main.css";

const app = createApp(App);

// Global event bus used to subscribe to websocket events
// such as message deletes, updates & truncation.
const eventBus = mitt();
app.provide("eventBus", eventBus);

app.use(router);
app.mount("#app");
