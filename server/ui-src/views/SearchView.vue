<script setup>
import AppLayout from "../components/AppLayout.vue";
import ListMessages from "../components/ListMessages.vue";
import NavSearch from "../components/NavSearch.vue";
import NavTags from "../components/NavTags.vue";
import Pagination from "../components/NavPagination.vue";
import SearchForm from "../components/SearchForm.vue";
import { useMessages } from "../composables/useMessages";
import { mailbox } from "../stores/mailbox";
import { pagination } from "../stores/pagination";
import { inject, onMounted, onUnmounted, watch } from "vue";
import { useRoute, useRouter } from "vue-router";

const route = useRoute();
const router = useRouter();
const eventBus = inject("eventBus");

const { loading, resolve, getSearch, apiURI, loadMessages } = useMessages();

let delayedRefresh = false;

function doSearch() {
	const s = getSearch();
	if (!s) {
		mailbox.searching = false;
		router.push("/");
		return;
	}

	mailbox.searching = s;
	apiURI.value = resolve("/api/v1/search") + "?query=" + encodeURIComponent(s);
	if (mailbox.timeZone !== "" && (s.indexOf("after:") !== -1 || s.indexOf("before:") !== -1)) {
		apiURI.value += "&tz=" + encodeURIComponent(mailbox.timeZone);
	}
	loadMessages();
}

function handleWSUpdate(data) {
	for (let x = 0; x < mailbox.messages.length; x++) {
		if (mailbox.messages[x].ID === data.ID) {
			mailbox.messages[x] = { ...mailbox.messages[x], ...data };
			return;
		}
	}
}

function handleWSDelete(data) {
	let removed = 0;
	for (let x = 0; x < mailbox.messages.length; x++) {
		if (mailbox.messages[x].ID === data.ID) {
			mailbox.messages.splice(x, 1);
			removed++;
			continue;
		}
	}
	if (!removed || delayedRefresh) return;
	delayedRefresh = true;
	window.setTimeout(() => {
		delayedRefresh = false;
		loadMessages();
	}, 500);
}

function handleWSTruncate() {
	router.push("/");
}

watch(
	() => route.fullPath,
	() => doSearch(),
);

onMounted(() => {
	mailbox.searching = getSearch();
	doSearch();
	eventBus.on("update", handleWSUpdate);
	eventBus.on("delete", handleWSDelete);
	eventBus.on("truncate", handleWSTruncate);
});

onUnmounted(() => {
	eventBus.off("update", handleWSUpdate);
	eventBus.off("delete", handleWSDelete);
	eventBus.off("truncate", handleWSTruncate);
});
</script>

<template>
	<AppLayout
		active-tab=""
		offcanvas-id="offcanvas"
		offcanvas-title="MessagePit"
		:loading="loading"
		@brand-click="pagination.start = 0"
	>
		<template #search>
			<SearchForm @load-messages="loadMessages" />
		</template>

		<template #sidebar>
			<NavSearch @load-messages="loadMessages" />
			<NavTags />
		</template>

		<template #modals>
			<NavSearch modals @load-messages="loadMessages" />
		</template>

		<div id="message-page" class="flex-grow-1 overflow-y-auto">
			<ListMessages :loading-messages="loading" />
		</div>
		<Pagination :total="mailbox.count" />
	</AppLayout>
</template>
