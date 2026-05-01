<script setup>
import { ref, computed, watch, onMounted } from "vue";
import { mailbox } from "../stores/mailbox.js";

const favicon = ref(false);
const iconPath = ref(false);
const iconTextColor = "#ffffff";
const iconBgColor = "#dd0000";
const iconFontSize = 40;
const iconProcessing = ref(false);
const iconTimeout = 500;

const count = computed(() => {
	let i = mailbox.unread;
	if (i > 1000) i = Math.floor(i / 1000) + "k";
	return i;
});

async function icoUpdate() {
	if (!favicon.value) return;

	if (!count.value) {
		iconProcessing.value = false;
		favicon.value.href = iconPath.value;
		return;
	}

	let fontSize = iconFontSize;
	let textPaddingX = 7;
	const textPaddingY = 3;

	const strlen = count.value.toString().length;

	if (strlen > 2) {
		textPaddingX = 4;
		fontSize = strlen > 3 ? 30 : 36;
	}

	const canvas = document.createElement("canvas");
	canvas.width = 64;
	canvas.height = 64;

	const ctx = canvas.getContext("2d");

	const icon = new Image();
	icon.src = iconPath.value;
	await icon.decode();

	ctx.drawImage(icon, 0, 0, 64, 64);

	ctx.font = `${fontSize}px Arial, sans-serif`;
	ctx.textAlign = "right";
	ctx.textBaseline = "top";
	const textMetrics = ctx.measureText(count.value);

	const paddingX = 7;
	const paddingY = 4;
	const cornerRadius = 8;

	const width = textMetrics.width + paddingX * 2;
	const height = fontSize + paddingY * 2;
	const x = canvas.width - width;
	const y = canvas.height - height - 1;

	ctx.fillStyle = iconBgColor;
	ctx.roundRect(x, y, width, height, cornerRadius);
	ctx.fill();

	ctx.fillStyle = iconTextColor;
	ctx.fillText(count.value, canvas.width - textPaddingX, canvas.height - fontSize - textPaddingY);

	iconProcessing.value = false;
	favicon.value.href = canvas.toDataURL("image/png");
}

watch(count, () => {
	if (!favicon.value || iconProcessing.value) return;
	iconProcessing.value = true;
	window.setTimeout(() => {
		icoUpdate();
	}, iconTimeout);
});

onMounted(() => {
	favicon.value = document.head.querySelector('link[rel="icon"]');
	if (favicon.value) {
		iconPath.value = favicon.value.href;
	}
});
</script>

<template><!-- renders nothing --></template>
