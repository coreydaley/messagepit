<script setup>
import AjaxLoader from "../AjaxLoader.vue";
import { ref } from "vue";
import { useCommon } from "../../composables/useCommon";
import { domToPng } from "modern-screenshot";
import DOMPurify from "dompurify";

const props = defineProps({
	message: {
		type: Object,
		default: () => ({}),
	},
});

const { resolve } = useCommon();

const html = ref(false);
const loading = ref(0);

function decodeEntities(s) {
	return new DOMParser().parseFromString(s, "text/html").body.textContent;
}

function initScreenshot() {
	loading.value = 1;
	const baseUrl = `${location.protocol}//${location.host}/`;
	const proxy = new URL(resolve("/proxy"), baseUrl).href;
	const urlRegex = /(url\(('|")?(https?:\/\/[^)'"]+)('|")?\))/gim;

	let h = props.message.HTML.replace(/<base .*>/im, "");

	h = h.replace(/<html [^>]+>/gim, "<html>");
	h = h.replace(/<o:p><\/o:p>/gm, "");
	h = h.replace(/<o:/gm, "<");
	h = h.replace(/<\/o:/gm, "</");

	h = DOMPurify.sanitize(h, {
		WHOLE_DOCUMENT: true,
		FORCE_BODY: false,
		ADD_TAGS: ["link", "meta", "o:p", "style"],
		ADD_ATTR: [
			"bordercolor",
			"charset",
			"content",
			"hspace",
			"http-equiv",
			"itemprop",
			"itemscope",
			"itemtype",
			"vertical-align",
			"vlink",
			"vspace",
			"xml:lang",
			"background",
		],
		FORBID_TAGS: ["script", "noscript"],
	});

	const doc = document.implementation.createHTMLDocument();
	doc.open();
	doc.writeln(h);
	doc.close();

	const styles = doc.getElementsByTagName("style");
	for (const i of styles) {
		i.innerHTML = i.innerHTML.replaceAll(urlRegex, (match, p1, p2, p3) => {
			if (typeof p2 === "string") {
				return `url(${p2}${proxy}?data=` + btoa(props.message.ID + ":" + decodeEntities(p3)) + `${p2})`;
			}
			return `url(${proxy}?data=` + btoa(props.message.ID + ":" + decodeEntities(p3)) + `)`;
		});
	}

	const stylesheets = doc.getElementsByTagName("link");
	for (const i of stylesheets) {
		const src = i.getAttribute("href");
		if (src && src.match(/^https?:\/\//i) && src.indexOf(window.location.origin + window.location.pathname) !== 0) {
			i.setAttribute("href", `${proxy}?data=` + btoa(props.message.ID + ":" + decodeEntities(src)));
		}
	}

	const images = doc.getElementsByTagName("img");
	for (const i of images) {
		const src = i.getAttribute("src");
		if (src && src.match(/^https?:\/\//i) && src.indexOf(window.location.origin + window.location.pathname) !== 0) {
			i.setAttribute("src", `${proxy}?data=` + btoa(props.message.ID + ":" + decodeEntities(src)));
		}
	}

	const backgrounds = doc.querySelectorAll("[background]");
	for (const i of backgrounds) {
		const src = i.getAttribute("background");

		if (src && src.match(/^https?:\/\//i) && src.indexOf(window.location.origin + window.location.pathname) !== 0) {
			i.setAttribute("background", `${proxy}?data=` + btoa(props.message.ID + ":" + decodeEntities(src)));
		}
	}

	html.value = new XMLSerializer().serializeToString(doc);
}

function doScreenshot() {
	let width = document.getElementById("message-view").getBoundingClientRect().width;

	const prev = document.getElementById("preview-html");
	if (prev && prev.getBoundingClientRect().width) {
		width = prev.getBoundingClientRect().width;
	}

	if (width < 300) {
		width = 300;
	}

	const i = document.getElementById("screenshot-html");

	i.style.width = width + "px";

	const body = i.contentWindow.document.querySelector("body");

	body.style.padding = "20px";

	domToPng(body, {
		backgroundColor: "#ffffff",
		height: i.contentWindow.document.body.scrollHeight,
		width,
		style: {
			margin: "0",
		},
	}).then((dataUrl) => {
		const link = document.createElement("a");
		link.download = props.message.ID + ".png";
		link.href = dataUrl;
		link.click();
		loading.value = 0;
		html.value = false;
	});
}

defineExpose({ initScreenshot });
</script>

<template>
	<iframe
		v-if="html"
		id="screenshot-html"
		:srcdoc="html"
		frameborder="0"
		style="position: absolute; margin-left: -100000px"
		@load="doScreenshot"
	>
	</iframe>

	<AjaxLoader :loading="loading" />
</template>
