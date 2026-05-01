<script setup>
import { ref, computed } from "vue";
import { mailbox } from "../stores/mailbox";
import { pagination } from "../stores/pagination";
import { useCommon } from "../composables/useCommon";
import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";

dayjs.extend(relativeTime);

const props = defineProps({
	loadingMessages: {
		type: Number,
		default: 0,
	},
	messages: {
		type: Array,
		default: null,
	},
	routeBase: {
		type: String,
		default: "/view/",
	},
	emptyText: {
		type: String,
		default: "No messages in your mailbox",
	},
});

const { colorHash, getFileSize, getSearch } = useCommon();

const tick = ref(0);

setInterval(() => {
	tick.value++;
}, 30000);

const msgList = computed(() => {
	return props.messages !== null ? props.messages : mailbox.messages;
});

function getRelativeCreated(message) {
	const d = new Date(message.Created);
	return dayjs(d).fromNow();
}

function getPrimaryEmailTo(message) {
	if (message.To && message.To.length > 0) {
		return message.To[0].Address;
	}

	return "[ Undisclosed recipients ]";
}

function isSelected(id) {
	return mailbox.selected.indexOf(id) !== -1;
}

function toggleSelected(e, id) {
	e.preventDefault();

	if (isSelected(id)) {
		mailbox.selected = mailbox.selected.filter((ele) => {
			return ele !== id;
		});
	} else {
		mailbox.selected.push(id);
	}
}

function selectRange(e, id) {
	e.preventDefault();

	let selecting = false;
	const lastSelected = mailbox.selected.length > 0 && mailbox.selected[mailbox.selected.length - 1];
	if (lastSelected === id) {
		mailbox.selected = mailbox.selected.filter((ele) => {
			return ele !== id;
		});
		return;
	}

	if (lastSelected === false) {
		mailbox.selected.push(id);
		return;
	}

	for (const d of mailbox.messages) {
		if (selecting) {
			if (!isSelected(d.ID)) {
				mailbox.selected.push(d.ID);
			}
			if (d.ID === lastSelected || d.ID === id) {
				break;
			}
		} else if (d.ID === id || d.ID === lastSelected) {
			if (!isSelected(d.ID)) {
				mailbox.selected.push(d.ID);
			}
			selecting = true;
		}
	}
}

function toTagUrl(t) {
	if (t.match(/ /)) {
		t = `"${t}"`;
	}
	const p = {
		q: "tag:" + t,
	};
	if (pagination.limit !== pagination.defaultLimit) {
		p.limit = pagination.limit.toString();
	}
	const params = new URLSearchParams(p);
	return "/search?" + params.toString();
}
</script>

<template>
	<!-- tick drives relative time updates -->
	<div :data-tick="tick" class="d-none"></div>
	<template v-if="msgList && msgList.length">
		<div class="list-group my-2">
			<RouterLink
				v-for="message in msgList"
				:id="message.ID"
				:key="'message_' + message.ID"
				:to="routeBase + message.ID"
				class="row gx-1 message d-flex small list-group-item list-group-item-action border-start-0 border-end-0"
				:class="[message.Read ? 'read' : '', isSelected(message.ID) ? ' selected' : '']"
				@click.meta="toggleSelected($event, message.ID)"
				@click.ctrl="toggleSelected($event, message.ID)"
				@click.shift="selectRange($event, message.ID)"
			>
				<div class="col-lg-3">
					<div class="d-lg-none float-end text-muted text-nowrap small">
						<i v-if="message.Attachments" class="bi bi-paperclip h6 me-1"></i>
						{{ getRelativeCreated(message) }}
					</div>
					<div v-if="message.From" class="overflow-x-hidden">
						<div class="text-truncate privacy">
							<b :title="'From: ' + message.From.Address">
								{{ message.From.Name ? message.From.Name : message.From.Address }}
							</b>
						</div>
					</div>
					<div class="overflow-x-hidden">
						<div class="text-truncate text-muted small privacy">
							To: {{ getPrimaryEmailTo(message) }}
							<span v-if="message.To && message.To.length > 1"> [+{{ message.To.length - 1 }}] </span>
						</div>
					</div>
				</div>
				<div class="col-lg-6 col-xxl-7 mt-2 mt-lg-0">
					<div class="subject text-truncate text-spaces-nowrap">
						<b>{{ message.Subject !== "" ? message.Subject : "[ no subject ]" }}</b>
					</div>
					<div v-if="message.Snippet !== ''" class="small text-muted text-truncate">
						{{ message.Snippet }}
					</div>
					<div v-if="message.Tags.length">
						<RouterLink
							v-for="t in message.Tags"
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
				</div>
				<div class="d-none d-lg-block col-1 small text-end text-muted">
					<i v-if="message.Attachments" class="bi bi-paperclip float-start h6"></i>
					{{ getFileSize(message.Size) }}
				</div>
				<div class="d-none d-lg-block col-2 col-xxl-1 small text-end text-muted">
					{{ getRelativeCreated(message) }}
				</div>
			</RouterLink>
		</div>
	</template>
	<template v-else>
		<p class="text-center mt-5">
			<span v-if="loadingMessages > 0" class="text-muted"> Loading messages... </span>
			<template v-else-if="getSearch()"
				>No results for <code>{{ getSearch() }}</code></template
			>
			<template v-else>{{ emptyText }}</template>
		</p>
	</template>
</template>
