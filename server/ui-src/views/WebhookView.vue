<script>
import About from "../components/AppAbout.vue";
import AjaxLoader from "../components/AjaxLoader.vue";
import CommonMixins from "../mixins/CommonMixins";
import { webhooksStore } from "../stores/webhooks";
import dayjs from "dayjs";

export default {
	components: {
		About,
		AjaxLoader,
	},

	mixins: [CommonMixins],

	inject: ["eventBus"],

	data() {
		return {
			webhooksStore,
			message: false,
			messagesList: [],
			errorMessage: false,
			replayURL: "",
			replayStatus: null,
			replayPending: false,
		};
	},

	computed: {
		previousID() {
			const l = this.messagesList.length;
			if (!this.message || !l) return false;
			let id = false;
			for (let x = 0; x < l; x++) {
				if (this.messagesList[x].ID === this.message.ID) return id;
				id = this.messagesList[x].ID;
			}
			return false;
		},

		nextID() {
			const l = this.messagesList.length;
			if (!this.message || !l) return false;
			let id = false;
			for (let x = l - 1; x > 0; x--) {
				if (this.messagesList[x].ID === this.message.ID) return id;
				id = this.messagesList[x].ID;
			}
			return id;
		},

		fullURL() {
			if (!this.message) return "";
			return this.message.Query
				? this.message.Path + "?" + this.message.Query
				: this.message.Path;
		},

		bodyLanguage() {
			const ct = (this.message && this.message.ContentType) || "";
			if (ct.includes("json")) return "json";
			if (ct.includes("xml")) return "xml";
			if (ct.includes("html")) return "html";
			return "plaintext";
		},

		prettyBody() {
			if (!this.message || !this.message.Body) return "";
			if (this.bodyLanguage === "json") {
				try {
					return JSON.stringify(JSON.parse(this.message.Body), null, 2);
				} catch {
					return this.message.Body;
				}
			}
			return this.message.Body;
		},

		sortedHeaders() {
			if (!this.message || !this.message.Headers) return [];
			return Object.entries(this.message.Headers)
				.sort(([a], [b]) => a.localeCompare(b))
				.map(([name, values]) => ({ name, value: values.join(", ") }));
		},

		queryParams() {
			if (!this.message || !this.message.Query) return [];
			const params = new URLSearchParams(this.message.Query);
			const result = [];
			for (const [k, v] of params.entries()) {
				result.push({ name: k, value: v });
			}
			return result;
		},
	},

	watch: {
		$route() {
			this.loadMessage();
		},
	},

	created() {
		const relativeTime = require("dayjs/plugin/relativeTime");
		dayjs.extend(relativeTime);
	},

	mounted() {
		this.messagesList = [...(webhooksStore.messages || [])];
		if (!this.messagesList.length) {
			this.loadMessagesList();
		}
		this.loadMessage();
		this.refreshUI();
		this.eventBus.on("webhook", this.handleWSNew);
		this.eventBus.on("webhook_delete", this.handleWSDelete);
		this.eventBus.on("webhook_truncate", this.handleWSTruncate);
	},

	unmounted() {
		this.eventBus.off("webhook", this.handleWSNew);
		this.eventBus.off("webhook_delete", this.handleWSDelete);
		this.eventBus.off("webhook_truncate", this.handleWSTruncate);
	},

	methods: {
		loadMessage() {
			this.message = false;
			this.replayStatus = null;
			const id = this.$route.params.id;
			this.get(
				this.resolve(`/api/v1/webhook/${id}`),
				false,
				(response) => {
					this.errorMessage = false;
					this.message = response.data;
					if (webhooksStore.unread > 0) webhooksStore.unread--;
					this.$nextTick(() => this.scrollSidebarToCurrent());
				},
				() => {
					this.errorMessage = "Webhook not found";
				},
			);
		},

		loadMessagesList() {
			this.get(this.resolve("/api/v1/webhooks"), { limit: 50 }, (response) => {
				webhooksStore.total = response.data.total;
				webhooksStore.unread = response.data.unread;
				webhooksStore.messages = response.data.messages;
				this.messagesList = [...(webhooksStore.messages || [])];
			});
		},

		refreshUI() {
			window.setTimeout(() => {
				this.$forceUpdate();
				this.refreshUI();
			}, 30000);
		},

		getRelativeCreated(msg) {
			return dayjs(new Date(msg.Created)).fromNow();
		},

		getAbsoluteCreated(msg) {
			return dayjs(new Date(msg.Created)).format("ddd, D MMM YYYY, h:mm:ss a");
		},

		isActive(id) {
			return this.message && this.message.ID === id;
		},

		methodBadgeClass(method) {
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
		},

		scrollSidebarToCurrent() {
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
		},

		deleteMessage() {
			const id = this.message.ID;
			const goToID = this.nextID ? this.nextID : this.previousID;
			this.delete(this.resolve(`/api/v1/webhook/${id}`), {}, () => {
				if (goToID) {
					return this.$router.push(`/webhooks/view/${goToID}`);
				}
				return this.$router.push("/webhooks");
			});
		},

		replayWebhook() {
			if (!this.replayURL || !this.message) return;
			this.replayPending = true;
			this.replayStatus = null;

			const headers = { "Content-Type": this.message.ContentType || "application/octet-stream" };
			fetch(this.replayURL, {
				method: this.message.Method,
				headers,
				body: ["GET", "HEAD", "OPTIONS"].includes(this.message.Method) ? undefined : this.message.Body,
			})
				.then((r) => {
					this.replayStatus = { ok: r.ok, code: r.status, text: r.statusText };
				})
				.catch((e) => {
					this.replayStatus = { ok: false, code: 0, text: e.message };
				})
				.finally(() => {
					this.replayPending = false;
				});
		},

		handleWSNew(data) {
			this.messagesList.unshift(data);
			webhooksStore.messages.unshift(data);
		},

		handleWSDelete(id) {
			this.messagesList = this.messagesList.filter((m) => m.ID !== id);
			webhooksStore.messages = webhooksStore.messages.filter((m) => m.ID !== id);
			if (this.message && this.message.ID === id) {
				this.$router.push("/webhooks");
			}
		},

		handleWSTruncate() {
			this.messagesList = [];
			this.$router.push("/webhooks");
		},
	},
};
</script>

<template>
	<div class="navbar navbar-expand-lg row flex-shrink-0 bg-primary text-white d-print-none" data-bs-theme="dark">
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
		<div v-if="!errorMessage" class="col-auto col-lg-4 col-xl-4 d-flex align-items-center justify-content-end gap-1">
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
								<span class="text-truncate flex-grow-1" :class="msg.Read ? '' : 'fw-semibold'">{{ msg.Path }}</span>
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
						<!-- Method + path title -->
						<h5 class="mb-3 font-monospace d-flex align-items-baseline gap-2 flex-wrap">
							<span class="badge fs-6" :class="methodBadgeClass(message.Method)">{{ message.Method }}</span>
							<span class="text-break">{{ fullURL }}</span>
						</h5>

						<!-- Summary table -->
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

						<!-- Query params -->
						<template v-if="queryParams.length">
							<h6 class="text-muted mb-2">Query Parameters</h6>
							<table class="table table-sm table-borderless small mb-3">
								<tbody>
									<tr v-for="p in queryParams" :key="p.name">
										<th class="text-muted fw-normal font-monospace" style="width: 40%; word-break: break-all">
											{{ p.name }}
										</th>
										<td class="font-monospace" style="word-break: break-all">{{ p.value }}</td>
									</tr>
								</tbody>
							</table>
						</template>

						<!-- Headers -->
						<h6 class="text-muted mb-2">Headers</h6>
						<div class="card mb-3">
							<div class="card-body p-0">
								<table class="table table-sm small mb-0">
									<tbody>
										<tr v-for="h in sortedHeaders" :key="h.name">
											<td class="text-muted font-monospace" style="width: 40%; word-break: break-all">
												{{ h.name }}
											</td>
											<td class="font-monospace" style="word-break: break-all">{{ h.value }}</td>
										</tr>
									</tbody>
								</table>
							</div>
						</div>

						<!-- Body -->
						<template v-if="message.Body">
							<h6 class="text-muted mb-2">Body</h6>
							<div class="card mb-3">
								<div class="card-body p-0">
									<pre
										class="mb-0 p-3 small"
										style="white-space: pre-wrap; word-break: break-all; max-height: 40rem; overflow-y: auto"
									>{{ prettyBody }}</pre>
								</div>
							</div>
						</template>

						<!-- Replay -->
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
