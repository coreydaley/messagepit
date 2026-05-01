<script setup>
import { watch, onBeforeMount } from "vue";
import { useRoute } from "vue-router";
import Favicon from "./components/AppFavicon.vue";
import AppBadge from "./components/AppBadge.vue";
import Notifications from "./components/AppNotifications.vue";
import EditTags from "./components/EditTags.vue";
import { useCommon } from "./composables/useCommon";
import { mailbox } from "./stores/mailbox";
import { smsStore } from "./stores/sms";
import { webhooksStore } from "./stores/webhooks";

const route = useRoute();
const { get, resolve, hideNav } = useCommon();

watch(route, () => {
	hideNav();
});

onBeforeMount(() => {
	get(resolve("/api/v1/webui"), false, (response) => {
		mailbox.uiConfig = response.data;

		if (mailbox.uiConfig.Label) {
			document.title = document.title + " - " + mailbox.uiConfig.Label;
		} else {
			document.title = document.title + " - " + location.hostname;
		}
	});

	get(resolve("/api/v1/sms/messages"), { limit: 1 }, (response) => {
		smsStore.total = response.data.total;
		smsStore.unread = response.data.unread;
	});

	get(resolve("/api/v1/webhooks"), { limit: 1 }, (response) => {
		webhooksStore.total = response.data.total;
		webhooksStore.unread = response.data.unread;
	});
});
</script>

<template>
	<RouterView />
	<Favicon />
	<AppBadge />
	<Notifications />
	<EditTags />
</template>
