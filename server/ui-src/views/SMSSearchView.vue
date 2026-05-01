<script>
import About from "../components/AppAbout.vue";
import AjaxLoader from "../components/AjaxLoader.vue";
import CommonMixins from "../mixins/CommonMixins";
import ListMessages from "../components/ListMessages.vue";
import Pagination from "../components/NavPagination.vue";
import { mailbox } from "../stores/mailbox";
import { pagination } from "../stores/pagination";
import { smsStore } from "../stores/sms";
import { webhooksStore } from "../stores/webhooks";

export default {
	components: {
		About,
		AjaxLoader,
		ListMessages,
		Pagination,
	},

	mixins: [CommonMixins],

	inject: ["eventBus"],

	data() {
		return {
			mailbox,
			pagination,
			smsStore,
			webhooksStore,
			search: "",
			results: [],
			total: 0,
		};
	},

	computed: {
		normalizedMessages() {
			return this.results.map((msg) => ({
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

	watch: {
		$route() {
			this.doSearch();
		},
	},

	mounted() {
		this.doSearch();
		this.eventBus.on("sms_delete", this.handleWSDelete);
		this.eventBus.on("sms_truncate", this.handleWSTruncate);
	},

	unmounted() {
		this.eventBus.off("sms_delete", this.handleWSDelete);
		this.eventBus.off("sms_truncate", this.handleWSTruncate);
	},

	methods: {
		doSearch() {
			const q = this.getSearch();
			if (!q) {
				this.$router.push("/sms");
				return;
			}
			this.search = q;

			const p = this.getPaginationParams();
			if (p?.start) {
				pagination.start = p.start;
			} else {
				pagination.start = 0;
			}
			if (p?.limit) {
				pagination.limit = p.limit;
			}

			this.get(
				this.resolve("/api/v1/sms/search"),
				{ query: q, start: pagination.start, limit: pagination.limit },
				(response) => {
					this.results = response.data.messages || [];
					this.total = response.data.total;
					pagination.start = response.data.start;
				},
			);
		},

		submitSearch(e) {
			e.preventDefault();
			if (this.search.trim()) {
				this.$router.push("/sms/search?q=" + encodeURIComponent(this.search.trim()));
			} else {
				this.$router.push("/sms");
			}
		},

		resetSearch() {
			this.$router.push("/sms");
		},

		handleWSDelete(id) {
			this.results = this.results.filter((m) => m.ID !== id);
		},

		handleWSTruncate() {
			this.$router.push("/sms");
		},
	},
};
</script>

<template>
	<div class="navbar navbar-expand-lg row flex-shrink-0 bg-primary text-white d-print-none" data-bs-theme="dark">
		<div class="col-xl-2 col-md-3 col-auto pe-0">
			<RouterLink to="/" class="navbar-brand text-white me-0">
				<i class="bi bi-funnel-fill"></i>
				<span class="ms-2 d-none d-sm-inline">MessagePit</span>
			</RouterLink>
		</div>
		<div class="col col-md-4 col-lg-5 col-xl-6 d-flex align-items-center gap-3">
			<div class="nav nav-pills flex-shrink-0">
				<RouterLink to="/" class="nav-link text-white opacity-75 px-3">
					<i class="bi bi-envelope-fill me-1"></i>
					Email
					<span v-if="mailbox.unread" class="badge rounded-pill ms-1 bg-white text-dark">
						{{ formatNumber(mailbox.unread) }}
					</span>
				</RouterLink>
				<RouterLink to="/sms" class="nav-link text-white px-3 active bg-white bg-opacity-25">
					<i class="bi bi-chat-fill me-1"></i>
					SMS
					<span v-if="smsStore.unread" class="badge rounded-pill ms-1 bg-white text-dark">
						{{ formatNumber(smsStore.unread) }}
					</span>
				</RouterLink>
				<RouterLink to="/webhooks" class="nav-link text-white opacity-75 px-3">
					<i class="bi bi-arrow-left-right me-1"></i>
					Webhooks
					<span v-if="webhooksStore.unread" class="badge rounded-pill ms-1 bg-white text-dark">
						{{ formatNumber(webhooksStore.unread) }}
					</span>
				</RouterLink>
			</div>
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
						<span v-if="search" class="btn btn-link position-absolute end-0 text-muted" @click="resetSearch">
							<i class="bi bi-x-circle"></i>
						</span>
					</div>
					<button class="btn btn-outline-secondary" type="submit">
						<i class="bi bi-search"></i>
					</button>
				</div>
			</form>
		</div>
		<div class="col-12 col-md-auto col-lg-4 col-xl-4 d-flex align-items-center justify-content-end mt-2 mt-md-0">
			<div class="me-auto d-md-none">
				<button
					class="btn btn-outline-light me-2"
					type="button"
					data-bs-toggle="offcanvas"
					data-bs-target="#smsSearchOffcanvas"
					aria-controls="smsSearchOffcanvas"
				>
					<i class="bi bi-list"></i>
				</button>
			</div>
			<About navbar />
		</div>
	</div>

	<div
		id="smsSearchOffcanvas"
		class="offcanvas-md offcanvas-start d-md-none"
		data-bs-scroll="true"
		tabindex="-1"
		aria-labelledby="smsSearchOffcanvasLabel"
	>
		<div class="offcanvas-header">
			<h5 id="smsSearchOffcanvasLabel" class="offcanvas-title">SMS Search</h5>
			<button
				type="button"
				class="btn-close"
				data-bs-dismiss="offcanvas"
				data-bs-target="#smsSearchOffcanvas"
				aria-label="Close"
			></button>
		</div>
		<div class="offcanvas-body pb-0">
			<div class="d-flex flex-column h-100">
				<div class="flex-grow-1 overflow-y-auto me-n3 pe-3">
					<div class="list-group my-2">
						<RouterLink to="/sms" class="list-group-item list-group-item-action">
							<i class="bi bi-arrow-left me-1"></i>
							All SMS
						</RouterLink>
					</div>
				</div>

			</div>
		</div>
	</div>

	<div class="row flex-fill" style="min-height: 0">
		<div class="d-none d-md-flex h-100 col-xl-2 col-md-3 flex-column">
			<div class="flex-grow-1 overflow-y-auto me-n3 pe-3">
				<div class="list-group my-2">
					<RouterLink to="/sms" class="list-group-item list-group-item-action">
						<i class="bi bi-arrow-left me-1"></i>
						All SMS
					</RouterLink>
					<div v-if="total" class="list-group-item disabled small text-muted">
						{{ formatNumber(total) }} result{{ total !== 1 ? "s" : "" }}
					</div>
				</div>
			</div>

		</div>

		<div class="col-xl-10 col-md-9 d-flex flex-column mh-100 ps-0 ps-md-2 pe-0">
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
		</div>
	</div>

	<About modals />
	<AjaxLoader :loading="loading" />
</template>
