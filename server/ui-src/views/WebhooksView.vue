<script setup>
import AppLayout from "../components/AppLayout.vue";
import Pagination from "../components/NavPagination.vue";
import { useCommon } from "../composables/useCommon";
import { pagination } from "../stores/pagination";
import { webhooksStore } from "../stores/webhooks";
import dayjs from "dayjs";
import { inject, onMounted, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";

const route = useRoute();
const router = useRouter();
const eventBus = inject("eventBus");

const { loading, resolve, formatNumber, get, del, getPaginationParams } = useCommon();

const search = ref("");
const tick = ref(0);
let tickTimer = null;

function getRelativeCreated(msg) {
	return tick.value >= 0 ? dayjs(new Date(msg.Created)).fromNow() : "";
}

function methodBadgeClass(method) {
	const map = {
		GET: "text-bg-success",
		POST: "text-bg-primary",
		PUT: "text-bg-warning",
		PATCH: "text-bg-info",
		DELETE: "text-bg-danger",
		HEAD: "text-bg-secondary",
		OPTIONS: "text-bg-secondary",
	};
	return map[method] || "text-bg-secondary";
}

function loadWebhooks() {
	const p = getPaginationParams();
	if (p?.start) {
		pagination.start = p.start;
	} else {
		pagination.start = 0;
	}
	if (p?.limit) {
		pagination.limit = p.limit;
	}
	get(resolve("/api/v1/webhooks"), { start: pagination.start, limit: pagination.limit }, (response) => {
		webhooksStore.total = response.data.total;
		webhooksStore.unread = response.data.unread;
		webhooksStore.messages = response.data.messages;
		pagination.start = response.data.start;
	});
}

function deleteAll() {
	del(resolve("/api/v1/webhooks"), {}, () => {
		webhooksStore.messages = [];
		webhooksStore.total = 0;
		webhooksStore.unread = 0;
		pagination.start = 0;
	});
}

function handleWSNew(data) {
	if (pagination.start === 0) {
		webhooksStore.messages.unshift(data);
	}
}

function handleWSDelete(id) {
	webhooksStore.messages = webhooksStore.messages.filter((m) => m.ID !== id);
}

function handleWSTruncate() {
	pagination.start = 0;
	loadWebhooks();
}

function submitSearch(e) {
	e.preventDefault();
	if (search.value.trim()) {
		router.push("/webhooks/search?q=" + encodeURIComponent(search.value.trim()));
	}
}

function resetSearch() {
	search.value = "";
}

watch(
	() => route.fullPath,
	() => loadWebhooks(),
);

onMounted(() => {
	loadWebhooks();
	tickTimer = setInterval(() => {
		tick.value++;
	}, 30000);
	eventBus.on("webhook", handleWSNew);
	eventBus.on("webhook_delete", handleWSDelete);
	eventBus.on("webhook_truncate", handleWSTruncate);
});

onUnmounted(() => {
	clearInterval(tickTimer);
	eventBus.off("webhook", handleWSNew);
	eventBus.off("webhook_delete", handleWSDelete);
	eventBus.off("webhook_truncate", handleWSTruncate);
});
</script>

<template>
	<AppLayout active-tab="webhooks" offcanvas-id="webhooksOffcanvas" offcanvas-title="Webhooks" :loading="loading">
		<template #search>
			<form class="flex-fill" @submit="submitSearch">
				<div class="input-group flex-nowrap">
					<div class="ms-md-2 d-flex border bg-body rounded-start flex-fill position-relative">
						<input
							v-model.trim="search"
							type="text"
							class="form-control border-0"
							aria-label="Search webhooks"
							placeholder="Search requests"
						/>
						<span
							v-if="search"
							class="btn btn-link position-absolute end-0 text-muted"
							@click="resetSearch"
						>
							<i class="bi bi-x-circle"></i>
						</span>
					</div>
					<button class="btn btn-outline-secondary" type="submit">
						<i class="bi bi-search"></i>
					</button>
				</div>
			</form>
		</template>

		<template #sidebar>
			<div class="list-group my-2">
				<button class="list-group-item list-group-item-action active" disabled>
					<i class="bi bi-arrow-left-right me-1"></i>
					Webhooks
					<span v-if="webhooksStore.unread" class="badge rounded-pill ms-1 float-end text-bg-secondary">
						{{ formatNumber(webhooksStore.unread) }}
					</span>
				</button>
				<button
					class="list-group-item list-group-item-action"
					:disabled="!webhooksStore.total"
					@click="deleteAll"
				>
					<i class="bi bi-trash-fill me-1 text-danger"></i>
					Delete all
				</button>
			</div>
		</template>

		<div id="webhook-list" class="flex-grow-1 overflow-y-auto">
			<template v-if="!webhooksStore.messages.length">
				<p class="text-center text-muted mt-5">No webhook requests captured</p>
			</template>
			<template v-else>
				<div class="list-group list-group-flush">
					<RouterLink
						v-for="msg in webhooksStore.messages"
						:key="msg.ID"
						:to="'/webhooks/view/' + msg.ID"
						class="row gx-1 d-flex small list-group-item list-group-item-action message py-2 px-3"
						:class="msg.Read ? 'read' : ''"
					>
						<div class="col-12 d-flex align-items-center gap-2 overflow-x-hidden">
							<span class="badge font-monospace flex-shrink-0" :class="methodBadgeClass(msg.Method)">
								{{ msg.Method }}
							</span>
							<span class="text-truncate flex-grow-1" :class="msg.Read ? '' : 'fw-bold'">{{
								msg.Path
							}}</span>
							<span class="text-nowrap text-muted ms-auto">{{ getRelativeCreated(msg) }}</span>
						</div>
						<div class="col-12 d-flex gap-2 mt-1 overflow-x-hidden">
							<span class="text-truncate text-muted flex-grow-1">
								<template v-if="msg.Snippet">{{ msg.Snippet }}</template>
								<template v-else-if="msg.ContentType">{{ msg.ContentType }}</template>
								<template v-else><em>no body</em></template>
							</span>
							<span class="text-nowrap text-muted">{{ msg.SourceIP }}</span>
						</div>
					</RouterLink>
				</div>
			</template>
		</div>
		<Pagination :total="webhooksStore.total" :count="webhooksStore.messages.length" />
	</AppLayout>
</template>
