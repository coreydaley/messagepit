<script setup>
import { ref, watch } from "vue";
import { mailbox } from "../stores/mailbox.js";

const updating = ref(false);
const needsUpdate = ref(false);
const timeout = 500;

function updateAppBadge() {
	if (!("setAppBadge" in navigator)) return;
	navigator.setAppBadge(mailbox.unread);
}

function scheduleUpdate() {
	updating.value = true;
	needsUpdate.value = false;

	window.setTimeout(() => {
		updateAppBadge();
		updating.value = false;

		if (needsUpdate.value) {
			scheduleUpdate();
		}
	}, timeout);
}

watch(
	() => mailbox.unread,
	() => {
		if (updating.value) {
			needsUpdate.value = true;
			return;
		}
		scheduleUpdate();
	},
	{ immediate: true },
);
</script>

<template><!-- renders nothing --></template>
