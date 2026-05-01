<script>
import AjaxLoader from "./AjaxLoader.vue";
import Settings from "./AppSettings.vue";
import CommonMixins from "../mixins/CommonMixins";
import { mailbox } from "../stores/mailbox";

export default {
	components: {
		AjaxLoader,
		Settings,
	},

	mixins: [CommonMixins],

	props: {
		modals: {
			type: Boolean,
			default: false,
		},
		navbar: {
			type: Boolean,
			default: false,
		},
	},

	data() {
		return {
			mailbox,
		};
	},

	computed: {
		isEdgeBuild() {
			const re = /^(v\d+.\d+.\d+-)/i;
			return re.test(mailbox.appInfo.Version);
		},
	},

	methods: {
		loadInfo() {
			this.get(this.resolve("/api/v1/info"), false, (response) => {
				mailbox.appInfo = response.data;
				this.modal("AppInfoModal").show();
			});
		},

		requestNotifications() {
			// check if the browser supports notifications
			if (!("Notification" in window)) {
				alert("This browser does not support desktop notifications");
			}

			// we need to ask the user for permission
			else if (Notification.permission !== "denied") {
				Notification.requestPermission().then((permission) => {
					if (permission === "granted") {
						mailbox.notificationsEnabled = true;
					}

					this.modal("EnableNotificationsModal").hide();
				});
			}
		},
	},
};
</script>

<template>
	<template v-if="navbar">
		<button class="btn btn-sm text-white opacity-75 px-2" title="About MessagePit" @click="loadInfo()">
			<i class="bi bi-info-circle-fill"></i>
		</button>
		<RouterLink
			:to="resolve('/api/v1/')"
			target="_blank"
			class="btn btn-sm text-white opacity-75 px-2 no-icon"
			title="API documentation"
		>
			<i class="bi bi-book"></i>
		</RouterLink>
		<button
			v-if="mailbox.connected && mailbox.notificationsSupported && !mailbox.notificationsEnabled"
			class="btn btn-sm text-white opacity-75 px-2"
			data-bs-toggle="modal"
			data-bs-target="#EnableNotificationsModal"
			title="Enable browser notifications"
		>
			<i class="bi bi-bell"></i>
		</button>
		<button
			class="btn btn-sm text-white opacity-75 px-2"
			data-bs-toggle="modal"
			data-bs-target="#SettingsModal"
			title="MessagePit UI settings"
		>
			<i class="bi bi-gear-fill"></i>
		</button>
	</template>

	<template v-else-if="modals">
		<!-- Modals -->
		<div
			id="AppInfoModal"
			class="modal modal-xl fade"
			tabindex="-1"
			aria-labelledby="AppInfoModalLabel"
			aria-hidden="true"
		>
			<div class="modal-dialog">
				<div v-if="mailbox.appInfo.RuntimeStats" class="modal-content">
					<div class="modal-header">
						<h5 id="AppInfoModalLabel" class="modal-title">
							MessagePit
							<code>({{ mailbox.appInfo.Version }})</code>
							<span v-if="isEdgeBuild" class="badge bg-info text-dark ms-2">edge build</span>
						</h5>
						<button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
					</div>
					<div class="modal-body">
						<div class="row g-3">
							<div v-if="mailbox.appInfo.LatestVersion != 'disabled'" class="col-12">
								<div v-if="mailbox.appInfo.LatestVersion == ''">
									<div class="alert alert-warning mb-0">
										There might be a newer version available. The check failed.
									</div>
								</div>
								<div v-else-if="mailbox.appInfo.Version != mailbox.appInfo.LatestVersion">
									<a
										class="btn btn-warning d-block"
										:href="
											'https://github.com/coreydaley/messagepit/releases/tag/' +
											mailbox.appInfo.LatestVersion
										"
									>
										A new version of MessagePit ({{ mailbox.appInfo.LatestVersion }}) is available.
									</a>
								</div>
							</div>
							<div class="col-12">
								<RouterLink to="/api/v1/" class="btn btn-primary w-100" target="_blank">
									<i class="bi bi-braces"></i>
									OpenAPI / Swagger API documentation
								</RouterLink>
							</div>
							<div class="col-sm-6">
								<a
									class="btn btn-primary w-100"
									href="https://github.com/coreydaley/messagepit"
									target="_blank"
								>
									<i class="bi bi-github"></i>
									Github
								</a>
							</div>
							<div class="col-sm-6">
								<a
									class="btn btn-primary w-100"
									href="https://github.com/coreydaley/messagepit#readme"
									target="_blank"
								>
									Documentation
								</a>
							</div>
							<div class="col-4">
								<div class="card border-secondary text-center h-100">
									<div class="card-header small">Database size</div>
									<div class="card-body text-muted d-flex align-items-center justify-content-center">
										<h5 class="card-title mb-0">
											{{ getFileSize(mailbox.appInfo.DatabaseSize) }}
										</h5>
									</div>
								</div>
							</div>
							<div class="col-4">
								<div class="card border-secondary text-center h-100">
									<div class="card-header small">RAM usage</div>
									<div class="card-body text-muted d-flex align-items-center justify-content-center">
										<h5 class="card-title mb-0">
											{{ getFileSize(mailbox.appInfo.RuntimeStats.Memory) }}
										</h5>
									</div>
								</div>
							</div>
							<div class="col-4">
								<div class="card border-secondary text-center h-100">
									<div class="card-header small">Up since</div>
									<div class="card-body text-muted d-flex align-items-center justify-content-center">
										<h5 class="card-title mb-0">
											{{ secondsToRelative(mailbox.appInfo.RuntimeStats.Uptime) }}
										</h5>
									</div>
								</div>
							</div>
						</div>
					</div>
					<div class="modal-footer d-flex align-items-center">
						<p class="text-muted small mb-0 me-auto">
							Based on the excellent work of
							<a href="https://github.com/axllent/mailpit" target="_blank" class="text-muted">Mailpit</a>
							by
							<a href="https://github.com/axllent" target="_blank" class="text-muted">Ralph Slooten</a>.
						</p>
						<button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">Close</button>
					</div>
				</div>
			</div>
		</div>

		<div
			id="EnableNotificationsModal"
			class="modal fade"
			tabindex="-1"
			aria-labelledby="EnableNotificationsModalLabel"
			aria-hidden="true"
		>
			<div class="modal-dialog modal-lg">
				<div class="modal-content">
					<div class="modal-header">
						<h5 id="EnableNotificationsModalLabel" class="modal-title">Enable browser notifications?</h5>
						<button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
					</div>
					<div class="modal-body">
						<p class="h4">Get browser notifications when MessagePit receives new messages?</p>
						<p>
							Note that your browser will ask you for confirmation when you click
							<code>enable notifications</code>, and that you must have MessagePit open in a browser tab
							to be able to receive the notifications.
						</p>
					</div>
					<div class="modal-footer">
						<button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">Cancel</button>
						<button type="button" class="btn btn-success" @click="requestNotifications">
							Enable notifications
						</button>
					</div>
				</div>
			</div>
		</div>

		<Settings />
	</template>

	<AjaxLoader :loading="loading" />
</template>
