<script>
import About from "../components/AppAbout.vue";
import AjaxLoader from "../components/AjaxLoader.vue";
import CommonMixins from "../mixins/CommonMixins";
import ListMessages from "../components/ListMessages.vue";
import { mailbox } from "../stores/mailbox";
import { smsStore } from "../stores/sms";

export default {
	components: {
		About,
		AjaxLoader,
		ListMessages,
	},

	mixins: [CommonMixins],

	inject: ["eventBus"],

	data() {
		return {
			mailbox,
			smsStore,
		};
	},

	computed: {
		normalizedMessages() {
			return smsStore.messages.map((msg) => ({
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
			}));
		},
	},

	mounted() {
		this.loadSMS();
		this.eventBus.on("sms", this.handleWSNew);
		this.eventBus.on("sms_delete", this.handleWSDelete);
		this.eventBus.on("sms_truncate", this.handleWSTruncate);
	},

	unmounted() {
		this.eventBus.off("sms", this.handleWSNew);
		this.eventBus.off("sms_delete", this.handleWSDelete);
		this.eventBus.off("sms_truncate", this.handleWSTruncate);
	},

	methods: {
		loadSMS() {
			this.get(this.resolve("/api/v1/sms/messages"), { limit: 50 }, (response) => {
				smsStore.total = response.data.total;
				smsStore.unread = response.data.unread;
				smsStore.messages = response.data.messages;
			});
		},

		markAllRead() {
			for (const msg of smsStore.messages) {
				if (!msg.Read) {
					this.put(this.resolve(`/api/v1/sms/message/${msg.ID}/read`), {}, () => {});
					msg.Read = true;
				}
			}
			smsStore.unread = 0;
		},

		deleteAll() {
			this.delete(this.resolve("/api/v1/sms/messages"), {}, () => {
				smsStore.messages = [];
				smsStore.total = 0;
				smsStore.unread = 0;
			});
		},

		handleWSNew(data) {
			smsStore.messages.unshift(data);
		},

		handleWSDelete(id) {
			smsStore.messages = smsStore.messages.filter((m) => m.ID !== id);
		},

		handleWSTruncate() {
			this.loadSMS();
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
				<RouterLink to="/sms" class="nav-link text-white px-3 active bg-white bg-opacity-25">
					<i class="bi bi-chat-fill me-1"></i>
					SMS
					<span v-if="smsStore.unread" class="badge rounded-pill ms-1 text-bg-secondary">
						{{ formatNumber(smsStore.unread) }}
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
					data-bs-target="#smsOffcanvas"
					aria-controls="smsOffcanvas"
				>
					<i class="bi bi-list"></i>
				</button>
			</div>
		</div>
	</div>

	<div
		id="smsOffcanvas"
		class="offcanvas-md offcanvas-start d-md-none"
		data-bs-scroll="true"
		tabindex="-1"
		aria-labelledby="smsOffcanvasLabel"
	>
		<div class="offcanvas-header">
			<h5 id="smsOffcanvasLabel" class="offcanvas-title">SMS</h5>
			<button
				type="button"
				class="btn-close"
				data-bs-dismiss="offcanvas"
				data-bs-target="#smsOffcanvas"
				aria-label="Close"
			></button>
		</div>
		<div class="offcanvas-body pb-0">
			<div class="d-flex flex-column h-100">
				<div class="flex-grow-1 overflow-y-auto me-n3 pe-3">
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
						<button
							class="list-group-item list-group-item-action"
							:disabled="!smsStore.total"
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
		<div class="d-none d-md-flex h-100 col-xl-2 col-md-3 flex-column">
			<div class="flex-grow-1 overflow-y-auto me-n3 pe-3">
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
					<button
						class="list-group-item list-group-item-action"
						:disabled="!smsStore.total"
						@click="deleteAll"
					>
						<i class="bi bi-trash-fill me-1 text-danger"></i>
						Delete all
					</button>
				</div>
			</div>
			<About />
		</div>

		<div class="col-xl-10 col-md-9 mh-100 ps-0 ps-md-2 pe-0">
			<div id="message-page" class="mh-100" style="overflow-y: auto">
				<ListMessages
					:loading-messages="loading"
					:messages="normalizedMessages"
					route-base="/sms/view/"
					empty-text="No SMS messages"
				/>
			</div>
		</div>
	</div>

	<About modals />
	<AjaxLoader :loading="loading" />
</template>
