<script>
import About from "../components/AppAbout.vue";
import AjaxLoader from "../components/AjaxLoader.vue";
import CommonMixins from "../mixins/CommonMixins";
import { mailbox } from "../stores/mailbox";
import { smsStore } from "../stores/sms";
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
			mailbox,
			smsStore,
			webhooksStore,
		};
	},

	created() {
		const relativeTime = require("dayjs/plugin/relativeTime");
		dayjs.extend(relativeTime);
	},

	mounted() {
		this.loadWebhooks();
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
		loadWebhooks() {
			this.get(this.resolve("/api/v1/webhooks"), { limit: 50 }, (response) => {
				webhooksStore.total = response.data.total;
				webhooksStore.unread = response.data.unread;
				webhooksStore.messages = response.data.messages;
			});
		},

		deleteAll() {
			this.delete(this.resolve("/api/v1/webhooks"), {}, () => {
				webhooksStore.messages = [];
				webhooksStore.total = 0;
				webhooksStore.unread = 0;
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

		handleWSNew(data) {
			webhooksStore.messages.unshift(data);
		},

		handleWSDelete(id) {
			webhooksStore.messages = webhooksStore.messages.filter((m) => m.ID !== id);
		},

		handleWSTruncate() {
			this.loadWebhooks();
		},
	},
};
</script>

<template>
	<div class="navbar navbar-expand-lg row flex-shrink-0 bg-primary text-white d-print-none" data-bs-theme="dark">
		<div class="col-xl-2 col-md-3 col-auto pe-0">
			<RouterLink to="/" class="navbar-brand text-white me-0">
				<img :src="resolve('/mailpit.svg')" alt="MessagePit" />
				<span class="ms-2 d-none d-sm-inline">MessagePit</span>
			</RouterLink>
		</div>
		<div class="col col-md-4 col-lg-5 col-xl-6 d-flex align-items-center gap-3">
			<div class="nav nav-pills flex-shrink-0">
				<RouterLink to="/" class="nav-link text-white opacity-75 px-3">
					<i class="bi bi-envelope-fill me-1"></i>
					Email
					<span v-if="mailbox.unread" class="badge rounded-pill ms-1 text-bg-secondary">
						{{ formatNumber(mailbox.unread) }}
					</span>
				</RouterLink>
				<RouterLink to="/sms" class="nav-link text-white opacity-75 px-3">
					<i class="bi bi-chat-fill me-1"></i>
					SMS
					<span v-if="smsStore.unread" class="badge rounded-pill ms-1 text-bg-secondary">
						{{ formatNumber(smsStore.unread) }}
					</span>
				</RouterLink>
				<RouterLink to="/webhooks" class="nav-link text-white px-3 active bg-white bg-opacity-25">
					<i class="bi bi-arrow-left-right me-1"></i>
					Webhooks
					<span v-if="webhooksStore.unread" class="badge rounded-pill ms-1 text-bg-secondary">
						{{ formatNumber(webhooksStore.unread) }}
					</span>
				</RouterLink>
			</div>
		</div>
		<div class="col-12 col-md-auto col-lg-4 col-xl-4 text-end mt-2 mt-md-0">
			<div class="float-start d-md-none">
				<button
					class="btn btn-outline-light me-2"
					type="button"
					data-bs-toggle="offcanvas"
					data-bs-target="#webhooksOffcanvas"
					aria-controls="webhooksOffcanvas"
				>
					<i class="bi bi-list"></i>
				</button>
			</div>
		</div>
	</div>

	<div
		id="webhooksOffcanvas"
		class="offcanvas-md offcanvas-start d-md-none"
		data-bs-scroll="true"
		tabindex="-1"
		aria-labelledby="webhooksOffcanvasLabel"
	>
		<div class="offcanvas-header">
			<h5 id="webhooksOffcanvasLabel" class="offcanvas-title">Webhooks</h5>
			<button
				type="button"
				class="btn-close"
				data-bs-dismiss="offcanvas"
				data-bs-target="#webhooksOffcanvas"
				aria-label="Close"
			></button>
		</div>
		<div class="offcanvas-body pb-0">
			<div class="d-flex flex-column h-100">
				<div class="flex-grow-1 overflow-y-auto me-n3 pe-3">
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
				</div>
				<About />
			</div>
		</div>
	</div>

	<div class="row flex-fill" style="min-height: 0">
		<div class="d-none d-md-flex h-100 col-xl-3 col-md-3 flex-column">
			<div class="flex-grow-1 overflow-y-auto me-n3 pe-3">
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
			</div>
			<About />
		</div>

		<div class="col-xl-9 col-md-9 mh-100 ps-0 ps-md-2 pe-0">
			<div id="webhook-list" class="mh-100" style="overflow-y: auto">
				<template v-if="!webhooksStore.messages || !webhooksStore.messages.length">
					<p class="text-center text-muted mt-5">No webhook requests captured</p>
				</template>
				<template v-else>
					<div class="list-group list-group-flush">
						<RouterLink
							v-for="msg in webhooksStore.messages"
							:key="msg.ID"
							:to="'/webhooks/view/' + msg.ID"
							class="row gx-1 d-flex small list-group-item list-group-item-action py-2 px-3"
							:class="msg.Read ? 'read' : ''"
						>
							<div class="col-12 d-flex align-items-center gap-2 overflow-x-hidden">
								<span class="badge font-monospace flex-shrink-0" :class="methodBadgeClass(msg.Method)">
									{{ msg.Method }}
								</span>
								<span class="text-truncate flex-grow-1" :class="msg.Read ? '' : 'fw-bold'">{{ msg.Path }}</span>
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
		</div>
	</div>

	<About modals />
	<AjaxLoader :loading="loading" />
</template>
