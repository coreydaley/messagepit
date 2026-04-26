<script>
import About from "../components/AppAbout.vue";
import AjaxLoader from "../components/AjaxLoader.vue";
import CommonMixins from "../mixins/CommonMixins";
import { smsStore } from "../stores/sms";
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
			smsStore,
			message: false,
			messagesList: [],
			errorMessage: false,
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
		this.messagesList = [...smsStore.messages];
		if (!this.messagesList.length) {
			this.loadMessagesList();
		}
		this.loadMessage();
		this.refreshUI();
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
		loadMessage() {
			this.message = false;
			const id = this.$route.params.id;
			this.get(
				this.resolve(`/api/v1/sms/message/${id}`),
				false,
				(response) => {
					this.errorMessage = false;
					this.message = response.data;
					if (!this.message.Read) {
						this.put(this.resolve(`/api/v1/sms/message/${id}/read`), {}, () => {
							this.message.Read = true;
							this.handleWSUpdate({ ID: id, Read: true });
							if (smsStore.unread > 0) smsStore.unread--;
						});
					}
					this.$nextTick(() => this.scrollSidebarToCurrent());
				},
				() => {
					this.errorMessage = "Message not found";
				},
			);
		},

		loadMessagesList() {
			this.get(this.resolve("/api/v1/sms/messages"), { limit: 50 }, (response) => {
				smsStore.total = response.data.total;
				smsStore.unread = response.data.unread;
				smsStore.messages = response.data.messages;
				this.messagesList = [...smsStore.messages];
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
			return dayjs(new Date(msg.Created)).format("ddd, D MMM YYYY, h:mm a");
		},

		isActive(id) {
			return this.message && this.message.ID === id;
		},

		scrollSidebarToCurrent() {
			const cont = document.getElementById("SMSList");
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
			this.delete(this.resolve(`/api/v1/sms/message/${id}`), {}, () => {
				if (goToID) {
					return this.$router.push(`/sms/view/${goToID}`);
				}
				return this.$router.push("/sms");
			});
		},

		linkify(text) {
			const escaped = text
				.replace(/&/g, "&amp;")
				.replace(/</g, "&lt;")
				.replace(/>/g, "&gt;")
				.replace(/"/g, "&quot;");
			return escaped.replace(
				/(https?:\/\/[^\s]+)/g,
				'<a href="$1" target="_blank" rel="noopener noreferrer">$1</a>',
			);
		},

		handleWSNew(data) {
			this.messagesList.unshift(data);
			smsStore.messages.unshift(data);
		},

		handleWSDelete(id) {
			this.messagesList = this.messagesList.filter((m) => m.ID !== id);
			smsStore.messages = smsStore.messages.filter((m) => m.ID !== id);
			if (this.message && this.message.ID === id) {
				this.$router.push("/sms");
			}
		},

		handleWSTruncate() {
			this.messagesList = [];
			this.$router.push("/sms");
		},

		handleWSUpdate(data) {
			for (let i = 0; i < this.messagesList.length; i++) {
				if (this.messagesList[i].ID === data.ID) {
					this.messagesList[i] = { ...this.messagesList[i], ...data };
					break;
				}
			}
		},
	},
};
</script>

<template>
	<div class="navbar navbar-expand-lg row flex-shrink-0 bg-primary text-white d-print-none" data-bs-theme="dark">
		<div class="d-none d-xl-block col-xl-3 col-auto pe-0">
			<RouterLink to="/sms" class="navbar-brand text-white me-0">
				<img :src="resolve('/mailpit.svg')" alt="MessagePit" />
				<span class="ms-2 d-none d-sm-inline">MessagePit</span>
			</RouterLink>
		</div>
		<div v-if="!errorMessage" class="col col-xl-5">
			<RouterLink to="/sms" class="btn btn-outline-light me-3 d-xl-none" title="Return to SMS">
				<i class="bi bi-arrow-return-left"></i>
				<span class="ms-2 d-none d-lg-inline">Back</span>
			</RouterLink>
			<button class="btn btn-outline-light me-1 me-sm-2" title="Delete message" @click="deleteMessage()">
				<i class="bi bi-trash-fill me-md-2"></i>
				<span class="d-none d-md-inline">Delete</span>
			</button>
		</div>
		<div v-if="!errorMessage" class="col-auto col-lg-4 col-xl-4 text-end">
			<RouterLink
				:to="previousID ? '/sms/view/' + previousID : '/sms'"
				class="btn btn-outline-light ms-1 ms-sm-2 me-1"
				:class="previousID ? '' : 'disabled'"
				title="View previous message"
			>
				<i class="bi bi-caret-left-fill"></i>
			</RouterLink>
			<RouterLink
				:to="nextID ? '/sms/view/' + nextID : '/sms'"
				class="btn btn-outline-light"
				:class="nextID ? '' : 'disabled'"
				title="View next message"
			>
				<i class="bi bi-caret-right-fill"></i>
			</RouterLink>
		</div>
	</div>

	<div class="row flex-fill" style="min-height: 0">
		<div class="d-none d-xl-flex col-xl-3 h-100 flex-column">
			<div class="list-group my-2">
				<RouterLink to="/sms" class="list-group-item list-group-item-action">
					<i class="bi bi-arrow-return-left me-1"></i>
					<span class="ms-1">Return to SMS</span>
					<span
						v-if="smsStore.unread && !errorMessage"
						class="badge rounded-pill ms-1 float-end text-bg-secondary"
						title="Unread messages"
					>
						{{ formatNumber(smsStore.unread) }}
					</span>
				</RouterLink>
			</div>

			<div id="SMSList" class="flex-grow-1 overflow-y-auto px-1 me-n1">
				<template v-if="messagesList.length">
					<div class="list-group">
						<RouterLink
							v-for="msg in messagesList"
							:id="msg.ID"
							:key="'sms_' + msg.ID"
							:to="'/sms/view/' + msg.ID"
							class="row gx-1 message d-flex small list-group-item list-group-item-action message"
							:class="[msg.Read ? 'read' : '', isActive(msg.ID) ? 'active' : '']"
						>
							<div class="col overflow-x-hidden">
								<div class="text-truncate privacy small">
									<strong>{{ msg.From }}</strong>
								</div>
							</div>
							<div class="col-auto small">
								{{ getRelativeCreated(msg) }}
							</div>
							<div class="col-12 overflow-x-hidden">
								<div class="text-truncate privacy small">To: {{ msg.To }}</div>
							</div>
							<div class="col-12 overflow-x-hidden mt-1">
								<div class="text-truncate small">
									<b>{{ msg.Body || "[ no message ]" }}</b>
								</div>
							</div>
						</RouterLink>
					</div>
				</template>
			</div>

			<About />
		</div>

		<div class="col-xl-9 mh-100 ps-0 ps-md-2 pe-0">
			<div class="mh-100" style="overflow-y: auto">
				<template v-if="errorMessage">
					<h3 class="text-center my-3">{{ errorMessage }}</h3>
				</template>
				<template v-else-if="message">
					<div class="p-3 p-md-4">
						<table class="table table-sm table-borderless small mb-3">
							<tbody>
								<tr>
									<th class="text-muted fw-normal" style="width: 4rem">From</th>
									<td class="privacy">{{ message.From }}</td>
								</tr>
								<tr>
									<th class="text-muted fw-normal">To</th>
									<td class="privacy">{{ message.To }}</td>
								</tr>
								<tr>
									<th class="text-muted fw-normal">Date</th>
									<td>{{ getAbsoluteCreated(message) }}</td>
								</tr>
								<tr v-if="message.AccountSID">
									<th class="text-muted fw-normal">Account</th>
									<td class="text-muted font-monospace small">{{ message.AccountSID }}</td>
								</tr>
							</tbody>
						</table>

						<div class="card">
							<div class="card-body">
								<p
									class="mb-0 privacy"
									style="white-space: pre-wrap"
									v-html="linkify(message.Body)"
								></p>
							</div>
						</div>
					</div>
				</template>
			</div>
		</div>
	</div>

	<AjaxLoader :loading="loading" />
</template>
