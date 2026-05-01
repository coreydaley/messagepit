<script setup>
import AppLayout from "../components/AppLayout.vue";
import ListMessages from "../components/ListMessages.vue";
import Pagination from "../components/NavPagination.vue";
import { useCommon } from "../composables/useCommon";
import { pagination } from "../stores/pagination";
import { smsStore } from "../stores/sms";
import { computed, inject, onMounted, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";

const route = useRoute();
const router = useRouter();
const eventBus = inject("eventBus");

const { loading, resolve, formatNumber, get, put, del, getPaginationParams } = useCommon();

const search = ref("");

const normalizedMessages = computed(() =>
	(smsStore.messages || []).map((msg) => ({
		ID: msg.ID,
		Read: msg.Read,
		Created: msg.Created,
		From: { Name: "", Address: msg.From },
		To: [{ Address: msg.To, Name: "" }],
		Subject: msg.Body || "[ no message ]",
		Snippet: "",
		Tags: [],
		Attachments: 0,
		Size: msg.Body ? msg.Body.length : 0,
	})),
);

function loadSMS() {
	const p = getPaginationParams();
	if (p?.start) {
		pagination.start = p.start;
	} else {
		pagination.start = 0;
	}
	if (p?.limit) {
		pagination.limit = p.limit;
	}
	get(resolve("/api/v1/sms/messages"), { start: pagination.start, limit: pagination.limit }, (response) => {
		smsStore.total = response.data.total;
		smsStore.unread = response.data.unread;
		smsStore.messages = response.data.messages;
		pagination.start = response.data.start;
	});
}

function markAllRead() {
	for (const msg of smsStore.messages) {
		if (!msg.Read) {
			put(resolve(`/api/v1/sms/message/${msg.ID}/read`), {}, () => {});
			msg.Read = true;
		}
	}
	smsStore.unread = 0;
}

function deleteAll() {
	del(resolve("/api/v1/sms/messages"), {}, () => {
		smsStore.messages = [];
		smsStore.total = 0;
		smsStore.unread = 0;
		pagination.start = 0;
	});
}

function handleWSNew(data) {
	if (pagination.start === 0) {
		smsStore.messages.unshift(data);
	}
}

function handleWSDelete(id) {
	smsStore.messages = smsStore.messages.filter((m) => m.ID !== id);
}

function handleWSTruncate() {
	pagination.start = 0;
	loadSMS();
}

function submitSearch(e) {
	e.preventDefault();
	if (search.value.trim()) {
		router.push("/sms/search?q=" + encodeURIComponent(search.value.trim()));
	}
}

function resetSearch() {
	search.value = "";
}

watch(
	() => route.fullPath,
	() => loadSMS(),
);

onMounted(() => {
	loadSMS();
	eventBus.on("sms", handleWSNew);
	eventBus.on("sms_delete", handleWSDelete);
	eventBus.on("sms_truncate", handleWSTruncate);
});

onUnmounted(() => {
	eventBus.off("sms", handleWSNew);
	eventBus.off("sms_delete", handleWSDelete);
	eventBus.off("sms_truncate", handleWSTruncate);
});
</script>

<template>
	<AppLayout active-tab="sms" offcanvas-id="smsOffcanvas" offcanvas-title="SMS" :loading="loading">
		<template #search>
			<form class="flex-fill" @submit="submitSearch">
				<div class="input-group flex-nowrap">
					<div class="ms-md-2 d-flex border bg-body rounded-start flex-fill position-relative">
						<input
							v-model.trim="search"
							type="text"
							class="form-control border-0"
							aria-label="Search SMS"
							placeholder="Search messages"
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
					<i class="bi bi-chat-fill me-1"></i>
					SMS
					<span v-if="smsStore.unread" class="badge rounded-pill ms-1 float-end text-bg-secondary">
						{{ formatNumber(smsStore.unread) }}
					</span>
				</button>
				<button
					class="list-group-item list-group-item-action"
					:disabled="!smsStore.unread"
					@click="markAllRead"
				>
					<i class="bi bi-eye-fill me-1"></i>
					Mark all read
				</button>
				<button class="list-group-item list-group-item-action" :disabled="!smsStore.total" @click="deleteAll">
					<i class="bi bi-trash-fill me-1 text-danger"></i>
					Delete all
				</button>
			</div>
			<div v-if="smsStore.total" class="small text-muted mt-2 px-1">
				<div class="d-flex justify-content-between border-top pt-2 pb-1">
					<span>Total SMS</span>
					<strong class="text-body">{{ formatNumber(smsStore.total) }}</strong>
				</div>
				<div class="d-flex justify-content-between pb-1">
					<span>Unread</span>
					<strong class="text-body">{{ formatNumber(smsStore.unread) }}</strong>
				</div>
			</div>
		</template>

		<div id="message-page" class="flex-grow-1 overflow-y-auto">
			<ListMessages
				:loading-messages="loading"
				:messages="normalizedMessages"
				route-base="/sms/view/"
				empty-text="No SMS messages"
			/>
		</div>
		<Pagination :total="smsStore.total" :count="smsStore.messages.length" />
	</AppLayout>
</template>
