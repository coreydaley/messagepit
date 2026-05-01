<script setup>
import { ref, onMounted, inject } from "vue";
import { Toast } from "bootstrap";
import { mailbox } from "../stores/mailbox";
import { smsStore } from "../stores/sms";
import { webhooksStore } from "../stores/webhooks";
import { useCommon } from "../composables/useCommon";

const eventBus = inject("eventBus");
const { resolve } = useCommon();

const toastMessage = ref(false);
const reconnectRefresh = ref(false);
const socketURI = ref(false);
const socketLastConnection = ref(0);
const socketBreaks = ref(0);
const pauseNotifications = ref(false);
const version = ref(false);
const clientErrors = ref([]);

function browserNotify(title, message) {
	if (!("Notification" in window)) {
		return;
	}

	if (Notification.permission === "granted") {
		const options = {
			body: message,
			icon: resolve("/notification.png"),
		};

		(() => new Notification(title, options))();
	}
}

function setMessageToast(m) {
	if (mailbox.notificationsEnabled || toastMessage.value) {
		return;
	}

	toastMessage.value = m;

	const el = document.getElementById("messageToast");
	if (el) {
		el.addEventListener("hidden.bs.toast", () => {
			toastMessage.value = false;
		});

		Toast.getOrCreateInstance(el).show();
	}
}

function closeToast() {
	const el = document.getElementById("messageToast");
	if (el) {
		Toast.getOrCreateInstance(el).hide();
	}
}

function addClientError(d) {
	d.expire = Date.now() + 5000;
	clientErrors.value.push(d);
}

function errorNotificationCron() {
	window.setTimeout(() => {
		clientErrors.value.forEach((err, idx) => {
			if (err.expire < Date.now()) {
				clientErrors.value.splice(idx, 1);
			}
		});
		errorNotificationCron();
	}, 1000);
}

function socketBreakReset() {
	window.setTimeout(() => {
		socketBreaks.value = 0;
		socketBreakReset();
	}, 15000);
}

function connect() {
	const ws = new WebSocket(socketURI.value);
	ws.onmessage = (e) => {
		let response;
		try {
			response = JSON.parse(e.data);
		} catch {
			return;
		}

		if (response.Type === "new" && response.Data) {
			eventBus.emit("new", response.Data);

			for (const i in response.Data.Tags) {
				if (
					mailbox.tags.findIndex((e) => {
						return e.toLowerCase() === response.Data.Tags[i].toLowerCase();
					}) < 0
				) {
					mailbox.tags.push(response.Data.Tags[i]);
					mailbox.tags.sort((a, b) => {
						return a.toLowerCase().localeCompare(b.toLowerCase());
					});
				}
			}

			if (!pauseNotifications.value) {
				pauseNotifications.value = true;
				const from = response.Data.From !== null ? response.Data.From.Address : "[unknown]";
				const subject = String(response.Data.Subject ?? "").substring(0, 100);
				browserNotify("New mail from: " + from, subject);
				setMessageToast(response.Data);
				window.setTimeout(() => {
					pauseNotifications.value = false;
				}, 2000);
			}
		} else if (response.Type === "prune") {
			window.scrollInPlace = true;
			mailbox.refresh = true;
			window.setTimeout(() => {
				mailbox.refresh = false;
			}, 500);
			eventBus.emit("prune");
		} else if (response.Type === "stats" && response.Data) {
			mailbox.total = response.Data.Total;
			mailbox.unread = response.Data.Unread;

			if (version.value !== response.Data.Version) {
				location.reload();
			}
		} else if (response.Type === "delete" && response.Data) {
			eventBus.emit("delete", response.Data);
		} else if (response.Type === "update" && response.Data) {
			eventBus.emit("update", response.Data);
		} else if (response.Type === "truncate") {
			eventBus.emit("truncate");
		} else if (response.Type === "sms" && response.Data) {
			smsStore.total++;
			if (!response.Data.Read) {
				smsStore.unread++;
			}
			eventBus.emit("sms", response.Data);
		} else if (response.Type === "sms_delete" && response.Data) {
			smsStore.total = Math.max(0, smsStore.total - 1);
			eventBus.emit("sms_delete", response.Data);
		} else if (response.Type === "sms_truncate") {
			smsStore.total = 0;
			smsStore.unread = 0;
			eventBus.emit("sms_truncate");
		} else if (response.Type === "webhook" && response.Data) {
			webhooksStore.total++;
			if (!response.Data.Read) {
				webhooksStore.unread++;
			}
			eventBus.emit("webhook", response.Data);
		} else if (response.Type === "webhook_delete" && response.Data) {
			webhooksStore.total = Math.max(0, webhooksStore.total - 1);
			eventBus.emit("webhook_delete", response.Data);
		} else if (response.Type === "webhook_truncate") {
			webhooksStore.total = 0;
			webhooksStore.unread = 0;
			eventBus.emit("webhook_truncate");
		} else if (response.Type === "error") {
			addClientError(response.Data);
		}
	};

	ws.onopen = () => {
		mailbox.connected = true;
		smsStore.connected = true;
		socketLastConnection.value = Date.now();
		if (reconnectRefresh.value) {
			reconnectRefresh.value = false;
			mailbox.refresh = true;
			window.setTimeout(() => {
				mailbox.refresh = false;
			}, 500);
		}
	};

	ws.onclose = () => {
		if (socketLastConnection.value === 0) {
			console.log("Unable to connect to websocket, disabling websocket support");
			return;
		}

		if (mailbox.connected) {
			socketBreaks.value++;
		}

		mailbox.connected = false;
		smsStore.connected = false;

		if (socketBreaks.value > 3) {
			console.log("Unstable websocket connection, disabling websocket support");
			return;
		}
		if (Date.now() - socketLastConnection.value > 5000) {
			reconnectRefresh.value = true;
		} else {
			reconnectRefresh.value = false;
		}

		setTimeout(() => {
			connect();
		}, 1000);
	};

	ws.onerror = function () {
		ws.close();
	};
}

onMounted(() => {
	const d = document.getElementById("app");
	if (d) {
		version.value = d.dataset.version;
	}

	const proto = location.protocol === "https:" ? "wss" : "ws";
	socketURI.value = proto + "://" + document.location.host + resolve(`/api/events`);

	socketBreakReset();
	connect();

	mailbox.notificationsSupported =
		window.isSecureContext && "Notification" in window && Notification.permission !== "denied";
	mailbox.notificationsEnabled = mailbox.notificationsSupported && Notification.permission === "granted";

	errorNotificationCron();
});
</script>

<template>
	<div class="toast-container position-fixed bottom-0 end-0 p-3">
		<div
			v-for="(error, i) in clientErrors"
			:key="'error_' + i"
			class="toast show"
			role="alert"
			aria-live="assertive"
			aria-atomic="true"
		>
			<div class="toast-header">
				<svg
					class="bd-placeholder-img rounded me-2"
					width="20"
					height="20"
					xmlns="http://www.w3.org/2000/svg"
					aria-hidden="true"
					preserveAspectRatio="xMidYMid slice"
					focusable="false"
				>
					<rect width="100%" height="100%" :fill="error.Level === 'warning' ? '#ffc107' : '#dc3545'"></rect>
				</svg>
				<strong class="me-auto">{{ error.Type }}</strong>
				<small class="text-body-secondary">{{ error.IP }}</small>
				<button type="button" class="btn-close" data-bs-dismiss="toast" aria-label="Close"></button>
			</div>
			<div class="toast-body">
				{{ error.Message }}
			</div>
		</div>

		<div id="messageToast" class="toast" role="alert" aria-live="assertive" aria-atomic="true">
			<div v-if="toastMessage" class="toast-header">
				<i class="bi bi-envelope-exclamation-fill me-2"></i>
				<strong class="me-auto">
					<RouterLink :to="'/view/' + toastMessage.ID" @click="closeToast">New message</RouterLink>
				</strong>
				<button type="button" class="btn-close" data-bs-dismiss="toast" aria-label="Close"></button>
			</div>

			<div class="toast-body">
				<div>
					<RouterLink
						:to="'/view/' + toastMessage.ID"
						class="d-block text-truncate text-body-secondary"
						@click="closeToast"
					>
						<template v-if="toastMessage.Subject !== ''">{{ toastMessage.Subject }}</template>
						<template v-else> [ no subject ] </template>
					</RouterLink>
				</div>
			</div>
		</div>
	</div>
</template>
