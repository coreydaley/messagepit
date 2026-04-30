import { createRouter, createWebHistory } from "vue-router";
import MailboxView from "../views/MailboxView.vue";
import MessageView from "../views/MessageView.vue";
import NotFoundView from "../views/NotFoundView.vue";
import SearchView from "../views/SearchView.vue";
import SMSMailboxView from "../views/SMSMailboxView.vue";
import SMSMessageView from "../views/SMSMessageView.vue";
import WebhooksView from "../views/WebhooksView.vue";
import WebhookView from "../views/WebhookView.vue";

const d = document.getElementById("app");
let webroot = "/";
if (d) {
	webroot = d.dataset.webroot;
}

// paths are relative to webroot
const router = createRouter({
	history: createWebHistory(webroot),
	routes: [
		{
			path: "/",
			component: MailboxView,
		},
		{
			path: "/search",
			component: SearchView,
		},
		{
			path: "/view/:id",
			component: MessageView,
		},
		{
			path: "/sms",
			component: SMSMailboxView,
		},
		{
			path: "/sms/view/:id",
			component: SMSMessageView,
		},
		{
			path: "/webhooks",
			component: WebhooksView,
		},
		{
			path: "/webhooks/view/:id",
			component: WebhookView,
		},
		{
			path: "/:pathMatch(.*)*",
			name: "NotFound",
			component: NotFoundView,
		},
	],
});

export default router;
