<script setup>
import AppLayout from "../components/AppLayout.vue";
import ListMessages from "../components/ListMessages.vue";
import NavMailbox from "../components/NavMailbox.vue";
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

const { loading, resolve, getPaginationParams, apiURI, reloadMailbox, loadMessages } = useMessages();

let delayedRefresh = false;
let paginationDelayed = false;

function loadMailbox() {
	const params = getPaginationParams();
	if (params?.start) {
		pagination.start = params.start;
	} else {
		pagination.start = 0;
	}
	if (params?.limit) {
		pagination.limit = params.limit;
	}
	loadMessages();
}

function delayedPaginationUpdate() {
	if (paginationDelayed) return;
	paginationDelayed = true;
	window.setTimeout(() => {
		const path = route.path;
		const p = { ...route.query };
		if (pagination.start > 0) {
			p.start = pagination.start.toString();
		} else {
			delete p.start;
		}
		if (pagination.limit !== pagination.defaultLimit) {
			p.limit = pagination.limit.toString();
		} else {
			delete p.limit;
		}
		mailbox.autoPaginating = false;
		const params = new URLSearchParams(p);
		router.replace(path + "?" + params.toString());
		paginationDelayed = false;
	}, 500);
}

function handleWSNew(data) {
	if (pagination.start < 1) {
		mailbox.messages.unshift(data);
		if (mailbox.messages.length > pagination.limit) {
			mailbox.messages.pop();
		}
	} else {
		pagination.start++;
		delayedPaginationUpdate();
	}
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
	loadMessages();
}

watch(
	() => route.fullPath,
	() => loadMailbox(),
);

onMounted(() => {
	mailbox.searching = false;
	apiURI.value = resolve("/api/v1/messages");
	loadMailbox();
	eventBus.on("new", handleWSNew);
	eventBus.on("update", handleWSUpdate);
	eventBus.on("delete", handleWSDelete);
	eventBus.on("truncate", handleWSTruncate);
});

onUnmounted(() => {
	eventBus.off("new", handleWSNew);
	eventBus.off("update", handleWSUpdate);
	eventBus.off("delete", handleWSDelete);
	eventBus.off("truncate", handleWSTruncate);
});
</script>

<template>
	<AppLayout
		active-tab="email"
		offcanvas-id="offcanvas"
		offcanvas-title="MessagePit"
		:loading="loading"
		@brand-click="reloadMailbox"
	>
		<template #search>
			<SearchForm />
		</template>

		<template #sidebar>
			<NavMailbox @load-messages="loadMessages" />
			<NavTags />
		</template>

		<template #modals>
			<NavMailbox modals @load-messages="loadMessages" />
		</template>

		<div id="message-page" class="flex-grow-1 overflow-y-auto">
			<ListMessages :loading-messages="loading" />
		</div>
		<Pagination :total="mailbox.total" />
	</AppLayout>
</template>
