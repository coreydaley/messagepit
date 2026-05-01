<script setup>
import AjaxLoader from "./AjaxLoader.vue";
import { mailbox } from "../stores/mailbox";
import { useCommon } from "../composables/useCommon";

const emit = defineEmits(["loadMessages"]);

const { loading, del, put, resolve } = useCommon();

function loadMessages() {
	emit("loadMessages");
}

function markSelectedRead() {
	if (!mailbox.selected.length) {
		return false;
	}
	put(resolve(`/api/v1/messages`), { Read: true, IDs: mailbox.selected }, () => {
		window.scrollInPlace = true;
		loadMessages();
	});
}

function isSelected(id) {
	return mailbox.selected.indexOf(id) !== -1;
}

function markSelectedUnread() {
	if (!mailbox.selected.length) {
		return false;
	}
	put(resolve(`/api/v1/messages`), { Read: false, IDs: mailbox.selected }, () => {
		window.scrollInPlace = true;
		loadMessages();
	});
}

function deleteMessages() {
	const ids = JSON.parse(JSON.stringify(mailbox.selected));
	if (!ids.length) {
		return false;
	}

	del(resolve(`/api/v1/messages`), { IDs: ids }, () => {
		window.scrollInPlace = true;
		loadMessages();
	});
}

function selectedHasUnread() {
	if (!mailbox.selected.length) {
		return false;
	}
	for (const i in mailbox.messages) {
		if (isSelected(mailbox.messages[i].ID) && !mailbox.messages[i].Read) {
			return true;
		}
	}
	return false;
}

function selectedHasRead() {
	if (!mailbox.selected.length) {
		return false;
	}
	for (const i in mailbox.messages) {
		if (isSelected(mailbox.messages[i].ID) && mailbox.messages[i].Read) {
			return true;
		}
	}
	return false;
}
</script>

<template>
	<template v-if="mailbox.selected.length">
		<button
			class="list-group-item list-group-item-action"
			:disabled="!selectedHasUnread()"
			@click="markSelectedRead"
		>
			<i class="bi bi-eye-fill me-1"></i>
			Mark read
		</button>
		<button
			class="list-group-item list-group-item-action"
			:disabled="!selectedHasRead()"
			@click="markSelectedUnread"
		>
			<i class="bi bi-eye-slash me-1"></i>
			Mark unread
		</button>
		<button class="list-group-item list-group-item-action" @click="deleteMessages()">
			<i class="bi bi-trash-fill me-1 text-danger"></i>
			Delete selected
		</button>
		<button class="list-group-item list-group-item-action" @click="mailbox.selected = []">
			<i class="bi bi-x-circle me-1"></i>
			Cancel selection
		</button>
	</template>

	<AjaxLoader :loading="loading" />
</template>
