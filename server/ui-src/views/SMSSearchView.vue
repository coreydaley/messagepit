<script setup>
import AppLayout from "../components/AppLayout.vue";
import ListMessages from "../components/ListMessages.vue";
import Pagination from "../components/NavPagination.vue";
import { useCommon } from "../composables/useCommon";
import { pagination } from "../stores/pagination";
import { computed, inject, onMounted, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";

const route = useRoute();
const router = useRouter();
const eventBus = inject("eventBus");

const { loading, resolve, formatNumber, get, getSearch, getPaginationParams } = useCommon();

const search = ref("");
const results = ref([]);
const total = ref(0);

const normalizedMessages = computed(() =>
	results.value.map((msg) => ({
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

function doSearch() {
	const q = getSearch();
	if (!q) {
		router.push("/sms");
		return;
	}
	search.value = q;

	const p = getPaginationParams();
	if (p?.start) {
		pagination.start = p.start;
	} else {
		pagination.start = 0;
	}
	if (p?.limit) {
		pagination.limit = p.limit;
	}

	get(resolve("/api/v1/sms/search"), { query: q, start: pagination.start, limit: pagination.limit }, (response) => {
		results.value = response.data.messages || [];
		total.value = response.data.total;
		pagination.start = response.data.start;
	});
}

function submitSearch(e) {
	e.preventDefault();
	if (search.value.trim()) {
		router.push("/sms/search?q=" + encodeURIComponent(search.value.trim()));
	} else {
		router.push("/sms");
	}
}

function resetSearch() {
	router.push("/sms");
}

function handleWSDelete(id) {
	results.value = results.value.filter((m) => m.ID !== id);
}

function handleWSTruncate() {
	router.push("/sms");
}

watch(
	() => route.fullPath,
	() => doSearch(),
);

onMounted(() => {
	doSearch();
	eventBus.on("sms_delete", handleWSDelete);
	eventBus.on("sms_truncate", handleWSTruncate);
});

onUnmounted(() => {
	eventBus.off("sms_delete", handleWSDelete);
	eventBus.off("sms_truncate", handleWSTruncate);
});
</script>

<template>
	<AppLayout active-tab="sms" offcanvas-id="smsSearchOffcanvas" offcanvas-title="SMS Search" :loading="loading">
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
				<RouterLink to="/sms" class="list-group-item list-group-item-action">
					<i class="bi bi-arrow-left me-1"></i>
					All SMS
				</RouterLink>
				<div v-if="total" class="list-group-item disabled small text-muted">
					{{ formatNumber(total) }} result{{ total !== 1 ? "s" : "" }}
				</div>
			</div>
		</template>

		<div id="message-page" class="flex-grow-1 overflow-y-auto">
			<template v-if="!loading && !normalizedMessages.length">
				<p class="text-center text-muted mt-5">No results for "{{ search }}"</p>
			</template>
			<template v-else>
				<ListMessages
					:loading-messages="loading"
					:messages="normalizedMessages"
					route-base="/sms/view/"
					empty-text="No SMS messages"
				/>
			</template>
		</div>
		<Pagination :total="total" :count="results.length" />
	</AppLayout>
</template>
