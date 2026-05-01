<script setup>
import About from "./AppAbout.vue";
import AjaxLoader from "./AjaxLoader.vue";
import { mailbox } from "../stores/mailbox";
import { smsStore } from "../stores/sms";
import { webhooksStore } from "../stores/webhooks";

defineProps({
	activeTab: { type: String, default: "" },
	offcanvasId: { type: String, required: true },
	offcanvasTitle: { type: String, default: "MessagePit" },
	brandTo: { type: String, default: "/" },
	loading: { type: Number, default: 0 },
});

defineEmits(["brand-click"]);

function formatNumber(nr) {
	return new Intl.NumberFormat().format(nr);
}
</script>

<template>
	<div class="navbar navbar-expand-lg row flex-shrink-0 bg-primary text-white d-print-none" data-bs-theme="dark">
		<div class="col-xl-2 col-md-3 col-auto pe-0">
			<RouterLink :to="brandTo" class="navbar-brand text-white me-0" @click="$emit('brand-click')">
				<i class="bi bi-funnel-fill"></i>
				<span class="ms-2 d-none d-sm-inline">MessagePit</span>
			</RouterLink>
		</div>
		<div class="col col-md-4 col-lg-5 col-xl-6 d-flex align-items-center gap-3">
			<div v-if="activeTab" class="nav nav-pills flex-shrink-0">
				<RouterLink
					to="/"
					class="nav-link text-white px-3"
					:class="activeTab === 'email' ? 'active bg-white bg-opacity-25' : 'opacity-75'"
				>
					<i class="bi bi-envelope-fill me-1"></i>
					Email
					<span v-if="mailbox.unread" class="badge rounded-pill ms-1 bg-white text-dark">
						{{ formatNumber(mailbox.unread) }}
					</span>
				</RouterLink>
				<RouterLink
					to="/sms"
					class="nav-link text-white px-3"
					:class="activeTab === 'sms' ? 'active bg-white bg-opacity-25' : 'opacity-75'"
				>
					<i class="bi bi-chat-fill me-1"></i>
					SMS
					<span v-if="smsStore.unread" class="badge rounded-pill ms-1 bg-white text-dark">
						{{ formatNumber(smsStore.unread) }}
					</span>
				</RouterLink>
				<RouterLink
					to="/webhooks"
					class="nav-link text-white px-3"
					:class="activeTab === 'webhooks' ? 'active bg-white bg-opacity-25' : 'opacity-75'"
				>
					<i class="bi bi-arrow-left-right me-1"></i>
					Webhooks
					<span v-if="webhooksStore.unread" class="badge rounded-pill ms-1 bg-white text-dark">
						{{ formatNumber(webhooksStore.unread) }}
					</span>
				</RouterLink>
			</div>
			<slot name="search" />
		</div>
		<div class="col-12 col-md-auto col-lg-4 col-xl-4 d-flex align-items-center justify-content-end mt-2 mt-md-0">
			<div class="me-auto d-md-none">
				<button
					class="btn btn-outline-light me-2"
					type="button"
					data-bs-toggle="offcanvas"
					:data-bs-target="'#' + offcanvasId"
					:aria-controls="offcanvasId"
				>
					<i class="bi bi-list"></i>
				</button>
			</div>
			<About navbar />
		</div>
	</div>

	<div
		:id="offcanvasId"
		class="offcanvas-md offcanvas-start d-md-none"
		data-bs-scroll="true"
		tabindex="-1"
		:aria-labelledby="offcanvasId + 'Label'"
	>
		<div class="offcanvas-header">
			<h5 :id="offcanvasId + 'Label'" class="offcanvas-title">{{ offcanvasTitle }}</h5>
			<button
				type="button"
				class="btn-close"
				data-bs-dismiss="offcanvas"
				:data-bs-target="'#' + offcanvasId"
				aria-label="Close"
			></button>
		</div>
		<div class="offcanvas-body pb-0">
			<div class="d-flex flex-column h-100">
				<div class="flex-grow-1 overflow-y-auto me-n3 pe-3">
					<slot name="sidebar" />
				</div>
			</div>
		</div>
	</div>

	<div class="row flex-fill" style="min-height: 0">
		<div class="d-none d-md-flex h-100 col-xl-2 col-md-3 flex-column">
			<div class="flex-grow-1 overflow-y-auto me-n3 pe-3">
				<slot name="sidebar" />
			</div>
		</div>
		<div class="col-xl-10 col-md-9 d-flex flex-column mh-100 ps-0 ps-md-2 pe-0">
			<slot />
		</div>
	</div>

	<slot name="modals" />
	<About modals />
	<AjaxLoader :loading="loading" />
</template>
