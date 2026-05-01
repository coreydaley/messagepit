<script setup>
import { ref, computed, watch, onMounted, onUnmounted, inject, nextTick } from "vue";
import { useRoute, useRouter } from "vue-router";
import About from "../components/AppAbout.vue";
import AjaxLoader from "../components/AjaxLoader.vue";
import { useCommon } from "../composables/useCommon";
import { webhooksStore } from "../stores/webhooks";
import dayjs from "dayjs";

const route = useRoute();
const router = useRouter();
const eventBus = inject("eventBus");

const { loading, get, del, resolve, formatNumber } = useCommon();

const message = ref(false);
const messagesList = ref([]);
const errorMessage = ref(false);
const replayURL = ref("");
const replayStatus = ref(null);
const replayPending = ref(false);
const tick = ref(0);

const previousID = computed(() => {
	const l = messagesList.value.length;
	if (!message.value || !l) return false;
	let id = false;
	for (let x = 0; x < l; x++) {
		if (messagesList.value[x].ID === message.value.ID) return id;
		id = messagesList.value[x].ID;
	}
	return false;
});

const nextID = computed(() => {
	const l = messagesList.value.length;
	if (!message.value || !l) return false;
	let id = false;
	for (let x = l - 1; x > 0; x--) {
		if (messagesList.value[x].ID === message.value.ID) return id;
		id = messagesList.value[x].ID;
	}
	return id;
});

const fullURL = computed(() => {
	if (!message.value) return "";
	return message.value.Query ? message.value.Path + "?" + message.value.Query : message.value.Path;
});

const bodyLanguage = computed(() => {
	const ct = (message.value && message.value.ContentType) || "";
	if (ct.includes("json")) return "json";
	if (ct.includes("xml")) return "xml";
	if (ct.includes("html")) return "html";
	return "plaintext";
});

const prettyBody = computed(() => {
	if (!message.value || !message.value.Body) return "";
	if (bodyLanguage.value === "json") {
		try {
			return JSON.stringify(JSON.parse(message.value.Body), null, 2);
		} catch {
			return message.value.Body;
		}
	}
	return message.value.Body;
});

const sortedHeaders = computed(() => {
	if (!message.value || !message.value.Headers) return [];
	return Object.entries(message.value.Headers)
		.sort(([a], [b]) => a.localeCompare(b))
		.map(([name, values]) => ({ name, value: values.join(", ") }));
});

const queryParams = computed(() => {
	if (!message.value || !message.value.Query) return [];
	const params = new URLSearchParams(message.value.Query);
	const result = [];
	for (const [k, v] of params.entries()) {
		result.push({ name: k, value: v });
	}
	return result;
});

function loadMessage() {
	message.value = false;
	replayStatus.value = null;
	const id = route.params.id;
	get(
		resolve(`/api/v1/webhook/${id}`),
		false,
		(response) => {
			errorMessage.value = false;
			message.value = response.data;
			if (webhooksStore.unread > 0) webhooksStore.unread--;
			nextTick(() => scrollSidebarToCurrent());
		},
		() => {
			errorMessage.value = "Webhook not found";
		},
	);
}

function loadMessagesList() {
	get(resolve("/api/v1/webhooks"), { limit: 50 }, (response) => {
		webhooksStore.total = response.data.total;
		webhooksStore.unread = response.data.unread;
		webhooksStore.messages = response.data.messages;
		messagesList.value = [...(webhooksStore.messages || [])];
	});
}

function getRelativeCreated(msg) {
	return dayjs(new Date(msg.Created)).fromNow();
}

function getAbsoluteCreated(msg) {
	return dayjs(new Date(msg.Created)).format("ddd, D MMM YYYY, h:mm:ss a");
}

function isActive(id) {
	return message.value && message.value.ID === id;
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

function scrollSidebarToCurrent() {
	const cont = document.getElementById("WebhookList");
	if (!cont) return;
	const c = cont.querySelector(".active");
	if (c) {
		const outer = cont.getBoundingClientRect();
		const li = c.getBoundingClientRect();
		if (outer.top > li.top || outer.bottom < li.bottom) {
			c.scrollIntoView({ behavior: "smooth", block: "center", inline: "nearest" });
		}
	}
}

function deleteMessage() {
	const id = message.value.ID;
	const goToID = nextID.value ? nextID.value : previousID.value;
	del(resolve(`/api/v1/webhook/${id}`), {}, () => {
		if (goToID) return router.push(`/webhooks/view/${goToID}`);
		return router.push("/webhooks");
	});
}

function replayWebhook() {
	if (!replayURL.value || !message.value) return;
	replayPending.value = true;
	replayStatus.value = null;

	const headers = { "Content-Type": message.value.ContentType || "application/octet-stream" };
	fetch(replayURL.value, {
		method: message.value.Method,
		headers,
		body: ["GET", "HEAD", "OPTIONS"].includes(message.value.Method) ? undefined : message.value.Body,
	})
		.then((r) => {
			replayStatus.value = { ok: r.ok, code: r.status, text: r.statusText };
		})
		.catch((e) => {
			replayStatus.value = { ok: false, code: 0, text: e.message };
		})
		.finally(() => {
			replayPending.value = false;
		});
}

const handleWSNew = (data) => {
	messagesList.value.unshift(data);
	webhooksStore.messages.unshift(data);
};

const handleWSDelete = (id) => {
	messagesList.value = messagesList.value.filter((m) => m.ID !== id);
	webhooksStore.messages = webhooksStore.messages.filter((m) => m.ID !== id);
	if (message.value && message.value.ID === id) {
		router.push("/webhooks");
	}
};

const handleWSTruncate = () => {
	messagesList.value = [];
	router.push("/webhooks");
};

watch(route, () => {
	loadMessage();
});

let tickIntervalId;

onMounted(() => {
	messagesList.value = [...(webhooksStore.messages || [])];
	if (!messagesList.value.length) {
		loadMessagesList();
	}
	loadMessage();
	tickIntervalId = setInterval(() => {
		tick.value++;
	}, 30000);
	eventBus.on("webhook", handleWSNew);
	eventBus.on("webhook_delete", handleWSDelete);
	eventBus.on("webhook_truncate", handleWSTruncate);
});

onUnmounted(() => {
	clearInterval(tickIntervalId);
	eventBus.off("webhook", handleWSNew);
	eventBus.off("webhook_delete", handleWSDelete);
	eventBus.off("webhook_truncate", handleWSTruncate);
});
</script>

<template>
	<!-- tick drives relative time updates -->
	<div
		class="navbar navbar-expand-lg row flex-shrink-0 bg-primary text-white d-print-none"
		data-bs-theme="dark"
		:data-tick="tick"
	>
		<div class="d-none d-xl-block col-xl-3 col-auto pe-0">
			<RouterLink to="/webhooks" class="navbar-brand text-white me-0">
				<i class="bi bi-funnel-fill"></i>
				<span class="ms-2 d-none d-sm-inline">MessagePit</span>
			</RouterLink>
		</div>
		<div v-if="!errorMessage" class="col col-xl-5">
			<RouterLink to="/webhooks" class="btn btn-outline-light me-3 d-xl-none" title="Return to Webhooks">
				<i class="bi bi-arrow-return-left"></i>
				<span class="ms-2 d-none d-lg-inline">Back</span>
			</RouterLink>
			<button class="btn btn-outline-light me-1 me-sm-2" title="Delete" @click="deleteMessage()">
				<i class="bi bi-trash-fill me-md-2"></i>
				<span class="d-none d-md-inline">Delete</span>
			</button>
		</div>
		<div
			v-if="!errorMessage"
			class="col-auto col-lg-4 col-xl-4 d-flex align-items-center justify-content-end gap-1"
		>
			<RouterLink
				:to="previousID ? '/webhooks/view/' + previousID : '/webhooks'"
				class="btn btn-outline-light ms-1 ms-sm-2 me-1"
				:class="previousID ? '' : 'disabled'"
				title="View previous"
			>
				<i class="bi bi-caret-left-fill"></i>
			</RouterLink>
			<RouterLink
				:to="nextID ? '/webhooks/view/' + nextID : '/webhooks'"
				class="btn btn-outline-light"
				:class="nextID ? '' : 'disabled'"
				title="View next"
			>
				<i class="bi bi-caret-right-fill"></i>
			</RouterLink>
			<About navbar />
		</div>
	</div>

	<div class="row flex-fill" style="min-height: 0">
		<div class="d-none d-xl-flex col-xl-3 h-100 flex-column">
			<div class="list-group my-2">
				<RouterLink to="/webhooks" class="list-group-item list-group-item-action">
					<i class="bi bi-arrow-return-left me-1"></i>
					<span class="ms-1">Return to Webhooks</span>
					<span
						v-if="webhooksStore.unread && !errorMessage"
						class="badge rounded-pill ms-1 float-end text-bg-secondary"
						title="Unread requests"
					>
						{{ formatNumber(webhooksStore.unread) }}
					</span>
				</RouterLink>
			</div>

			<div id="WebhookList" class="flex-grow-1 overflow-y-auto px-1 me-n1">
				<template v-if="messagesList.length">
					<div class="list-group">
						<RouterLink
							v-for="msg in messagesList"
							:id="msg.ID"
							:key="'wh_' + msg.ID"
							:to="'/webhooks/view/' + msg.ID"
							class="row gx-1 d-flex small list-group-item list-group-item-action message"
							:class="[msg.Read ? 'read' : '', isActive(msg.ID) ? 'active' : '']"
						>
							<div class="col-12 d-flex align-items-center gap-1 overflow-x-hidden">
								<span class="badge font-monospace flex-shrink-0" :class="methodBadgeClass(msg.Method)">
									{{ msg.Method }}
								</span>
								<span class="text-truncate flex-grow-1" :class="msg.Read ? '' : 'fw-semibold'">{{
									msg.Path
								}}</span>
							</div>
							<div class="col-12 d-flex justify-content-between mt-1">
								<span class="text-truncate opacity-75">{{ msg.SourceIP }}</span>
								<span class="text-nowrap opacity-75">{{ getRelativeCreated(msg) }}</span>
							</div>
						</RouterLink>
					</div>
				</template>
			</div>
		</div>

		<div class="col-xl-9 mh-100 ps-0 ps-md-2 pe-0">
			<div class="mh-100" style="overflow-y: auto">
				<template v-if="errorMessage">
					<h3 class="text-center my-3">{{ errorMessage }}</h3>
				</template>
				<template v-else-if="message">
					<div class="p-3 p-md-4">
						<h5 class="mb-3 font-monospace d-flex align-items-baseline gap-2 flex-wrap">
							<span class="badge fs-6" :class="methodBadgeClass(message.Method)">{{
								message.Method
							}}</span>
							<span class="text-break">{{ fullURL }}</span>
						</h5>

						<table class="table table-sm table-borderless small mb-3">
							<tbody>
								<tr>
									<th class="text-muted fw-normal" style="width: 6rem">Received</th>
									<td>{{ getAbsoluteCreated(message) }}</td>
								</tr>
								<tr>
									<th class="text-muted fw-normal">Source IP</th>
									<td class="font-monospace">{{ message.SourceIP }}</td>
								</tr>
								<tr v-if="message.ContentType">
									<th class="text-muted fw-normal">Content-Type</th>
									<td class="font-monospace">{{ message.ContentType }}</td>
								</tr>
								<tr v-if="message.BodySize > 0">
									<th class="text-muted fw-normal">Body size</th>
									<td>{{ message.BodySize }} bytes</td>
								</tr>
							</tbody>
						</table>

						<template v-if="queryParams.length">
							<h6 class="text-muted mb-2">Query Parameters</h6>
							<table class="table table-sm table-borderless small mb-3">
								<tbody>
									<tr v-for="p in queryParams" :key="p.name">
										<th
											class="text-muted fw-normal font-monospace"
											style="width: 40%; word-break: break-all"
										>
											{{ p.name }}
										</th>
										<td class="font-monospace" style="word-break: break-all">{{ p.value }}</td>
									</tr>
								</tbody>
							</table>
						</template>

						<h6 class="text-muted mb-2">Headers</h6>
						<div class="card mb-3">
							<div class="card-body p-0">
								<table class="table table-sm small mb-0">
									<tbody>
										<tr v-for="h in sortedHeaders" :key="h.name">
											<td
												class="text-muted font-monospace"
												style="width: 40%; word-break: break-all"
											>
												{{ h.name }}
											</td>
											<td class="font-monospace" style="word-break: break-all">{{ h.value }}</td>
										</tr>
									</tbody>
								</table>
							</div>
						</div>

						<template v-if="message.Body">
							<h6 class="text-muted mb-2">Body</h6>
							<div class="card mb-3">
								<div class="card-body p-0">
									<pre
										class="mb-0 p-3 small"
										style="
											white-space: pre-wrap;
											word-break: break-all;
											max-height: 40rem;
											overflow-y: auto;
										"
										>{{ prettyBody }}</pre
									>
								</div>
							</div>
						</template>

						<h6 class="text-muted mb-2">Replay</h6>
						<div class="card mb-3">
							<div class="card-body">
								<div class="input-group">
									<input
										v-model="replayURL"
										type="url"
										class="form-control form-control-sm font-monospace"
										placeholder="https://your-service/webhook"
									/>
									<button
										class="btn btn-sm btn-outline-secondary"
										:disabled="!replayURL || replayPending"
										@click="replayWebhook"
									>
										<i class="bi bi-send me-1"></i>
										<span v-if="replayPending">Sending…</span>
										<span v-else>Replay</span>
									</button>
								</div>
								<div v-if="replayStatus" class="mt-2 small">
									<span :class="replayStatus.ok ? 'text-success' : 'text-danger'">
										<i
											class="bi"
											:class="replayStatus.ok ? 'bi-check-circle-fill' : 'bi-x-circle-fill'"
										></i>
										{{ replayStatus.code }} {{ replayStatus.text }}
									</span>
								</div>
							</div>
						</div>
					</div>
				</template>
			</div>
		</div>
	</div>

	<AjaxLoader :loading="loading" />
</template>
