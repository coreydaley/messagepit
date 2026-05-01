<script setup>
import { ref, computed, watch, onMounted, onUnmounted, inject, nextTick } from "vue";
import { useRoute, useRouter } from "vue-router";
import About from "../components/AppAbout.vue";
import AjaxLoader from "../components/AjaxLoader.vue";
import Message from "../components/message/MessageItem.vue";
import Release from "../components/message/MessageRelease.vue";
import Screenshot from "../components/message/MessageScreenshot.vue";
import { useCommon } from "../composables/useCommon";
import { mailbox } from "../stores/mailbox";
import { pagination } from "../stores/pagination";
import dayjs from "dayjs";

const route = useRoute();
const router = useRouter();
const eventBus = inject("eventBus");

const { loading, get, del, put, resolve, getFileSize, formatNumber, colorHash, attachmentIcon, modal } = useCommon();

const message = ref(false);
const loadReleaseModal = ref(false);
const errorMessage = ref(false);
const apiSideNavURI = ref(false);
const apiSideNavParams = ref(new URLSearchParams());
const messagesList = ref([]);
const liveLoaded = ref(0);
const scrollLoading = ref(false);
const canLoadMore = ref(true);
const tick = ref(0);

const MessageList = ref(null);
const ScreenshotRef = ref(null);
const ReleaseRef = ref(null);

const isRead = computed(() => {
	const l = messagesList.value.length;
	if (!message.value || !l) return true;
	for (let x = 0; x < l; x++) {
		if (messagesList.value[x].ID === message.value.ID) {
			return messagesList.value[x].Read;
		}
	}
	return true;
});

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

function loadMessage() {
	message.value = false;
	const uri = resolve("/api/v1/message/" + route.params.id);
	get(
		uri,
		false,
		(response) => {
			errorMessage.value = false;
			const d = response.data;

			handleWSUpdate({ ID: d.ID, Read: true });

			if (d.HTML && d.Inline) {
				for (const i in d.Inline) {
					const a = d.Inline[i];
					if (a.ContentID !== "") {
						const escapedCID = a.ContentID.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
						d.HTML = d.HTML.replace(
							new RegExp("(=[\"']?)(cid:" + escapedCID + ")([\"'|\\s|\\/|>|;])", "g"),
							"$1" + resolve("/api/v1/message/" + d.ID + "/part/" + a.PartID) + "$3",
						);
					}
					if (a.FileName.match(/^[a-zA-Z0-9_\-.]+$/)) {
						d.HTML = d.HTML.replace(
							new RegExp("(=[\"']?)(" + a.FileName + ")([\"|'|\\s|\\/|>|;])", "g"),
							"$1" + resolve("/api/v1/message/" + d.ID + "/part/" + a.PartID) + "$3",
						);
					}
				}
			}

			if (d.HTML && d.Attachments) {
				for (const i in d.Attachments) {
					const a = d.Attachments[i];
					if (a.ContentID !== "") {
						const escapedCID = a.ContentID.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
						d.HTML = d.HTML.replace(
							new RegExp("(=[\"']?)(cid:" + escapedCID + ")([\"'|\\s|\\/|>|;])", "g"),
							"$1" + resolve("/api/v1/message/" + d.ID + "/part/" + a.PartID) + "$3",
						);
					}
					if (a.FileName.match(/^[a-zA-Z0-9_\-.]+$/)) {
						d.HTML = d.HTML.replace(
							new RegExp("(=[\"']?)(" + a.FileName + ")([\"|'|\\s|\\/|>|;])", "g"),
							"$1" + resolve("/api/v1/message/" + d.ID + "/part/" + a.PartID) + "$3",
						);
					}
				}
			}

			message.value = d;

			nextTick(() => {
				scrollSidebarToCurrent();
			});
		},
		(error) => {
			errorMessage.value = true;
			if (error.response && error.response.data) {
				if (error.response.data.Error) {
					errorMessage.value = error.response.data.Error;
				} else {
					errorMessage.value = error.response.data;
				}
			} else if (error.request) {
				errorMessage.value = "Error sending data to the server. Please refresh the page.";
			} else {
				errorMessage.value = error.message;
			}
		},
	);
}

const handleWSNew = (data) => {
	if (mailbox.searching || liveLoaded.value >= 100) return;
	liveLoaded.value++;
	messagesList.value.unshift(data);
};

const handleWSUpdate = (data) => {
	for (let x = 0; x < messagesList.value.length; x++) {
		if (messagesList.value[x].ID === data.ID) {
			messagesList.value[x] = { ...messagesList.value[x], ...data };
			return;
		}
	}
};

const handleWSDelete = (data) => {
	for (let x = 0; x < messagesList.value.length; x++) {
		if (messagesList.value[x].ID === data.ID) {
			messagesList.value.splice(x, 1);
			return;
		}
	}
};

const handleWSTruncate = () => {
	router.push("/");
};

function sidebarVisible() {
	return MessageList.value && MessageList.value.offsetParent !== null;
}

function scrollSidebarToCurrent() {
	const cont = document.getElementById("MessageList");
	if (!cont) return;
	const c = cont.querySelector(".router-link-active");
	if (c) {
		const outer = cont.getBoundingClientRect();
		const li = c.getBoundingClientRect();
		if (outer.top > li.top || outer.bottom < li.bottom) {
			c.scrollIntoView({ behavior: "smooth", block: "center", inline: "nearest" });
		}
	}
}

function scrollHandler(e) {
	if (!canLoadMore.value || scrollLoading.value) return;
	const { scrollTop, offsetHeight, scrollHeight } = e.target;
	if (scrollTop + offsetHeight + 150 >= scrollHeight) {
		loadMore();
	}
}

function loadMore() {
	if (messagesList.value.length) {
		const oldest = messagesList.value[messagesList.value.length - 1].Created;
		apiSideNavParams.value.set("before", oldest);
	}

	scrollLoading.value = true;

	get(
		apiSideNavURI.value,
		apiSideNavParams.value,
		(response) => {
			if (response.data.messages.length) {
				messagesList.value.push(...response.data.messages);
			} else {
				canLoadMore.value = false;
			}
			nextTick(() => {
				scrollLoading.value = false;
			});
		},
		null,
		true,
	);
}

function initLoadMoreAPIParams() {
	let apiURI = resolve(`/api/v1/messages`);
	const p = {};

	if (mailbox.searching) {
		apiURI = resolve(`/api/v1/search`);
		p.query = mailbox.searching;
	}

	if (pagination.limit !== pagination.defaultLimit) {
		p.limit = pagination.limit.toString();
	}

	apiSideNavURI.value = apiURI;
	apiSideNavParams.value = new URLSearchParams(p);
}

function getRelativeCreated(message) {
	const d = new Date(message.Created);
	return dayjs(d).fromNow();
}

function getPrimaryEmailTo(msg) {
	if (msg.To && msg.To.length > 0) return msg.To[0].Address;
	return "[ Undisclosed recipients ]";
}

function isActive(id) {
	return message.value.ID === id;
}

function toTagUrl(t) {
	if (t.match(/ /)) t = `"${t}"`;
	const p = { q: "tag:" + t };
	if (pagination.limit !== pagination.defaultLimit) {
		p.limit = pagination.limit.toString();
	}
	const params = new URLSearchParams(p);
	return "/search?" + params.toString();
}

function downloadMessageBody(str, ext) {
	const dl = document.createElement("a");
	dl.href = "data:text/plain," + encodeURIComponent(str);
	dl.target = "_blank";
	dl.download = message.value.ID + "." + ext;
	dl.click();
}

function screenshotMessageHTML() {
	ScreenshotRef.value.initScreenshot();
}

function toggleRead() {
	if (!message.value) return false;
	const read = !isRead.value;
	const ids = [message.value.ID];
	const uri = resolve("/api/v1/messages");
	put(uri, { Read: read, IDs: ids }, () => {
		if (!sidebarVisible()) return goBack();
		handleWSUpdate({ ID: message.value.ID, Read: read });
	});
}

function deleteMessage() {
	const ids = [message.value.ID];
	const uri = resolve("/api/v1/messages");
	// capture goToID before deletion to prevent WS race
	const goToID = nextID.value ? nextID.value : previousID.value;

	del(uri, { IDs: ids }, () => {
		if (!sidebarVisible()) return goBack();
		if (goToID) return router.push("/view/" + goToID);
		return goBack();
	});
}

function goBack() {
	mailbox.lastMessage = route.params.id;

	if (mailbox.searching) {
		const p = { q: mailbox.searching };
		if (pagination.start > 0) p.start = pagination.start.toString();
		if (pagination.limit !== pagination.defaultLimit) p.limit = pagination.limit.toString();
		router.push("/search?" + new URLSearchParams(p).toString());
	} else {
		const p = {};
		if (pagination.start > 0) p.start = pagination.start.toString();
		if (pagination.limit !== pagination.defaultLimit) p.limit = pagination.limit.toString();
		if (p.start || p.limit) {
			router.push("/?" + new URLSearchParams(p).toString());
		} else {
			router.push("/");
		}
	}
}

function reloadWindow() {
	location.reload();
}

function initReleaseModal() {
	loadReleaseModal.value = false;
	nextTick(() => {
		loadReleaseModal.value = true;
		nextTick(() => {
			modal("ReleaseModal").show();
			window.setTimeout(() => {
				ReleaseRef.value.initTags();
			}, 250);
		});
	});
}

watch(route, () => {
	loadMessage();
});

let tickIntervalId;

onMounted(() => {
	initLoadMoreAPIParams();
	loadMessage();

	messagesList.value = JSON.parse(JSON.stringify(mailbox.messages));
	if (!messagesList.value.length) {
		loadMore();
	}

	tickIntervalId = setInterval(() => {
		tick.value++;
	}, 30000);

	eventBus.on("new", handleWSNew);
	eventBus.on("update", handleWSUpdate);
	eventBus.on("delete", handleWSDelete);
	eventBus.on("truncate", handleWSTruncate);
});

onUnmounted(() => {
	clearInterval(tickIntervalId);
	eventBus.off("new", handleWSNew);
	eventBus.off("update", handleWSUpdate);
	eventBus.off("delete", handleWSDelete);
	eventBus.off("truncate", handleWSTruncate);
});
</script>

<template>
	<!-- tick drives relative time updates without $forceUpdate -->
	<div
		class="navbar navbar-expand-lg row flex-shrink-0 bg-primary text-white d-print-none"
		data-bs-theme="dark"
		:data-tick="tick"
	>
		<div class="d-none d-xl-block col-xl-3 col-auto pe-0">
			<RouterLink to="/" class="navbar-brand text-white me-0" @click="pagination.start = 0">
				<i class="bi bi-funnel-fill"></i>
				<span class="ms-2 d-none d-sm-inline">MessagePit</span>
			</RouterLink>
		</div>
		<div v-if="!errorMessage" class="col col-xl-5">
			<button class="btn btn-outline-light me-3 d-xl-none" title="Return to messages" @click="goBack()">
				<i class="bi bi-arrow-return-left"></i>
				<span class="ms-2 d-none d-lg-inline">Back</span>
			</button>
			<button class="btn btn-outline-light me-1 me-sm-2" title="Mark unread" @click="toggleRead()">
				<i class="bi bi-eye-slash me-md-2" :class="isRead ? 'bi-eye-slash' : 'bi-eye'"></i>
				<span class="d-none d-md-inline">Mark <template v-if="isRead">un</template>read</span>
			</button>
			<button
				v-if="mailbox.uiConfig.MessageRelay && mailbox.uiConfig.MessageRelay.Enabled"
				class="btn btn-outline-light me-1 me-sm-2"
				title="Release message"
				@click="initReleaseModal()"
			>
				<i class="bi bi-send me-md-2"></i>
				<span class="d-none d-md-inline">Release</span>
			</button>
			<button class="btn btn-outline-light me-1 me-sm-2" title="Delete message" @click="deleteMessage()">
				<i class="bi bi-trash-fill me-md-2"></i>
				<span class="d-none d-md-inline">Delete</span>
			</button>
		</div>
		<div
			v-if="!errorMessage"
			class="col-auto col-lg-4 col-xl-4 d-flex align-items-center justify-content-end gap-1"
		>
			<div id="DownloadBtn" class="dropdown d-inline-block">
				<button
					type="button"
					class="btn btn-outline-light dropdown-toggle"
					data-bs-toggle="dropdown"
					aria-expanded="false"
				>
					<i class="bi bi-file-arrow-down-fill"></i>
					<span class="d-none d-md-inline ms-1">Download</span>
				</button>
				<ul class="dropdown-menu dropdown-menu-end">
					<li>
						<a
							:href="resolve('/api/v1/message/' + message.ID + '/raw?dl=1')"
							class="dropdown-item"
							title="Message source including headers, body and attachments"
						>
							Raw message
						</a>
					</li>
					<li v-if="message.HTML">
						<button class="dropdown-item" @click="downloadMessageBody(message.HTML, 'html')">
							HTML body
						</button>
					</li>
					<li v-if="message.HTML">
						<button class="dropdown-item" @click="screenshotMessageHTML()">HTML screenshot</button>
					</li>
					<li v-if="message.Text">
						<button class="dropdown-item" @click="downloadMessageBody(message.Text, 'txt')">
							Text body
						</button>
					</li>
					<template v-if="message.Attachments && message.Attachments.length">
						<li>
							<hr class="dropdown-divider" />
						</li>
						<li>
							<h6 class="dropdown-header">Attachments</h6>
						</li>
						<li v-for="part in message.Attachments" :key="part.PartID">
							<RouterLink
								:to="'/api/v1/message/' + message.ID + '/part/' + part.PartID"
								class="row m-0 dropdown-item d-flex"
								target="_blank"
								:title="part.FileName !== '' ? part.FileName : '[ unknown ]'"
								style="min-width: 350px"
							>
								<div class="col-auto p-0 pe-1">
									<i class="bi" :class="attachmentIcon(part)"></i>
								</div>
								<div class="col text-truncate p-0 pe-1">
									{{ part.FileName !== "" ? part.FileName : "[ unknown ]" }}
								</div>
								<div class="col-auto text-muted small p-0">
									{{ getFileSize(part.Size) }}
								</div>
							</RouterLink>
						</li>
					</template>
					<template v-if="message.Inline && message.Inline.length">
						<li>
							<hr class="dropdown-divider" />
						</li>
						<li>
							<h6 class="dropdown-header">Inline image<span v-if="message.Inline.length > 1">s</span></h6>
						</li>
						<li v-for="part in message.Inline" :key="part.PartID">
							<RouterLink
								:to="'/api/v1/message/' + message.ID + '/part/' + part.PartID"
								class="row m-0 dropdown-item d-flex"
								target="_blank"
								:title="part.FileName !== '' ? part.FileName : '[ unknown ]'"
								style="min-width: 350px"
							>
								<div class="col-auto p-0 pe-1">
									<i class="bi" :class="attachmentIcon(part)"></i>
								</div>
								<div class="col text-truncate p-0 pe-1">
									{{ part.FileName !== "" ? part.FileName : "[ unknown ]" }}
								</div>
								<div class="col-auto text-muted small p-0">
									{{ getFileSize(part.Size) }}
								</div>
							</RouterLink>
						</li>
					</template>
				</ul>
			</div>

			<RouterLink
				:to="'/view/' + previousID"
				class="btn btn-outline-light ms-1 ms-sm-2 me-1"
				:class="previousID ? '' : 'disabled'"
				title="View previous message"
			>
				<i class="bi bi-caret-left-fill"></i>
			</RouterLink>
			<RouterLink :to="'/view/' + nextID" class="btn btn-outline-light" :class="nextID ? '' : 'disabled'">
				<i class="bi bi-caret-right-fill" title="View next message"></i>
			</RouterLink>
			<About navbar />
		</div>
	</div>

	<div class="row flex-fill" style="min-height: 0">
		<div class="d-none d-xl-flex col-xl-3 h-100 flex-column">
			<div v-if="mailbox.uiConfig.Label" class="text-center badge text-bg-primary py-2 my-2 w-100">
				<div class="text-truncate fw-normal" style="line-height: 1rem">
					{{ mailbox.uiConfig.Label }}
				</div>
			</div>

			<div class="list-group my-2" :class="mailbox.uiConfig.Label ? 'mt-0' : ''">
				<button class="list-group-item list-group-item-action" @click="goBack()">
					<i class="bi bi-arrow-return-left me-1"></i>
					<span class="ms-1">
						Return to
						<template v-if="mailbox.searching">search</template>
						<template v-else>inbox</template>
					</span>
					<span
						v-if="mailbox.unread && !errorMessage"
						class="badge rounded-pill ms-1 float-end text-bg-secondary"
						title="Unread messages"
					>
						{{ formatNumber(mailbox.unread) }}
					</span>
				</button>
			</div>

			<div
				id="MessageList"
				ref="MessageList"
				class="flex-grow-1 overflow-y-auto px-1 me-n1"
				@scroll="scrollHandler"
			>
				<button v-if="liveLoaded >= 100" class="w-100 alert alert-warning small" @click="reloadWindow()">
					Reload to see newer messages
				</button>
				<template v-if="messagesList && messagesList.length">
					<div class="list-group">
						<RouterLink
							v-for="summary in messagesList"
							:id="summary.ID"
							:key="'summary_' + summary.ID"
							:to="'/view/' + summary.ID"
							class="row gx-1 message d-flex small list-group-item list-group-item-action message"
							:class="[summary.Read ? 'read' : '', isActive(summary.ID) ? 'active' : '']"
						>
							<div class="col overflow-x-hidden">
								<div class="text-truncate privacy small">
									<strong v-if="summary.From" :title="'From: ' + summary.From.Address">
										{{ summary.From.Name ? summary.From.Name : summary.From.Address }}
									</strong>
								</div>
							</div>
							<div class="col-auto small">
								<i v-if="summary.Attachments" class="bi bi-paperclip h6"></i>
								{{ getRelativeCreated(summary) }}
							</div>
							<div class="col-12 overflow-x-hidden">
								<div class="text-truncate privacy small">
									To: {{ getPrimaryEmailTo(summary) }}
									<span v-if="summary.To && summary.To.length > 1">
										[+{{ summary.To.length - 1 }}]
									</span>
								</div>
							</div>
							<div class="col-12 overflow-x-hidden mt-1">
								<div class="text-truncates small">
									<b>{{ summary.Subject !== "" ? summary.Subject : "[ no subject ]" }}</b>
								</div>
							</div>
							<div v-if="summary.Tags.length" class="col-12">
								<RouterLink
									v-for="t in summary.Tags"
									:key="t"
									class="badge me-1"
									:to="toTagUrl(t)"
									:style="
										mailbox.showTagColors
											? { backgroundColor: colorHash(t) }
											: { backgroundColor: '#6c757d' }
									"
									:title="'Filter messages tagged with ' + t"
									@click="pagination.start = 0"
								>
									{{ t }}
								</RouterLink>
							</div>
						</RouterLink>
					</div>
				</template>
			</div>
		</div>

		<div class="col-xl-9 mh-100 ps-0 ps-md-2 pe-0">
			<div id="message-page" class="mh-100" style="overflow-y: auto">
				<template v-if="errorMessage">
					<h3 class="text-center my-3">
						{{ errorMessage }}
					</h3>
				</template>
				<Message v-else-if="message" :key="message.ID" :message="message" />
			</div>
		</div>
	</div>

	<About modals />
	<AjaxLoader :loading="loading" />
	<Release
		v-if="mailbox.uiConfig.MessageRelay && loadReleaseModal"
		ref="ReleaseRef"
		:message="message"
		@delete="deleteMessage"
	/>
	<Screenshot v-if="message" ref="ScreenshotRef" :message="message" />
</template>
