import { ref, computed } from "vue";
import { useRouter } from "vue-router";
import axios from "axios";
import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import ColorHash from "color-hash";
import { Modal, Offcanvas } from "bootstrap";
import { limitOptions } from "../stores/pagination";

dayjs.extend(relativeTime);

class BootstrapElement {
	hide() {}
	show() {}
}

const _colorHash = new ColorHash({ lightness: 0.3, saturation: [0.35, 0.5, 0.65] });

export function useCommon() {
	const router = useRouter();
	const loading = ref(0);
	const tagColorCache = {};
	const copiedText = ref({});

	const copyToClipboardSupported = computed(() => !!navigator.clipboard);

	function resolve(u) {
		return router.resolve(u).href;
	}

	function searchURI(s) {
		return resolve("/search") + "?q=" + encodeURIComponent(s);
	}

	function getFileSize(bytes) {
		if (bytes === 0) return "0B";
		const i = Math.floor(Math.log(bytes) / Math.log(1024));
		return (bytes / Math.pow(1024, i)).toFixed(1) * 1 + " " + ["B", "kB", "MB", "GB", "TB"][i];
	}

	function formatNumber(nr) {
		return new Intl.NumberFormat().format(nr);
	}

	function messageDate(d) {
		return dayjs(d).format("ddd, D MMM YYYY, h:mm a");
	}

	function secondsToRelative(d) {
		return dayjs().subtract(d, "seconds").fromNow();
	}

	function tagEncodeURI(tag) {
		if (tag.match(/ /)) tag = `"${tag}"`;
		return encodeURIComponent(`tag:${tag}`);
	}

	function getSearch() {
		if (!window.location.search) return false;
		const urlParams = new URLSearchParams(window.location.search);
		const q = urlParams.get("q")?.trim();
		if (!q) return false;
		return q;
	}

	function getPaginationParams() {
		if (!window.location.search) return null;
		const urlParams = new URLSearchParams(window.location.search);
		const start = parseInt(urlParams.get("start")?.trim(), 10);
		const limit = parseInt(urlParams.get("limit")?.trim(), 10);
		return {
			start: Number.isInteger(start) && start >= 0 ? start : null,
			limit: limitOptions.includes(limit) ? limit : null,
		};
	}

	function modal(id) {
		const e = document.getElementById(id);
		if (e) return Modal.getOrCreateInstance(e);
		return new BootstrapElement();
	}

	function hideNav() {
		const e = document.getElementById("offcanvas");
		if (e) Offcanvas.getOrCreateInstance(e).hide();
	}

	function handleError(error) {
		if (error.response && error.response.data) {
			if (error.response.data.Error) {
				alert(error.response.data.Error);
			} else {
				alert(error.response.data);
			}
		} else if (error.request) {
			alert("Error sending data to the server. Please try again.");
		} else {
			alert(error.message);
		}
	}

	function get(url, values, callback, errorCallback, hideLoader) {
		if (!hideLoader) loading.value++;
		axios
			.get(url, { params: values })
			.then(callback)
			.catch((err) => {
				if (typeof errorCallback === "function") return errorCallback(err);
				handleError(err);
			})
			.then(() => {
				if (!hideLoader && loading.value > 0) loading.value--;
			});
	}

	function post(url, data, callback) {
		loading.value++;
		axios
			.post(url, data)
			.then(callback)
			.catch(handleError)
			.then(() => {
				if (loading.value > 0) loading.value--;
			});
	}

	function del(url, data, callback) {
		loading.value++;
		axios
			.delete(url, { data })
			.then(callback)
			.catch(handleError)
			.then(() => {
				if (loading.value > 0) loading.value--;
			});
	}

	function put(url, data, callback) {
		loading.value++;
		axios
			.put(url, data)
			.then(callback)
			.catch(handleError)
			.then(() => {
				if (loading.value > 0) loading.value--;
			});
	}

	function allAttachments(message) {
		const a = [];
		for (const i in message.Attachments) {
			message.Attachments[i].ContentDisposition = "Attachment";
			a.push(message.Attachments[i]);
		}
		for (const i in message.OtherParts) {
			message.OtherParts[i].ContentDisposition = "Other";
			a.push(message.OtherParts[i]);
		}
		for (const i in message.Inline) {
			message.Inline[i].ContentDisposition = "Inline";
			a.push(message.Inline[i]);
		}
		return a.length ? a : false;
	}

	function isImage(a) {
		return a.ContentType.match(/^image\//);
	}

	function attachmentIcon(a) {
		const ext = a.FileName.split(".").pop().toLowerCase();
		if (a.ContentType.match(/^image\//)) return "bi-file-image-fill";
		if (a.ContentType.match(/\/pdf$/) || ext === "pdf") return "bi-file-pdf-fill";
		if (["doc", "docx", "odt", "rtf"].includes(ext)) return "bi-file-word-fill";
		if (["xls", "xlsx", "ods"].includes(ext)) return "bi-file-spreadsheet-fill";
		if (["ppt", "pptx", "key", "odp"].includes(ext)) return "bi-file-slides-fill";
		if (["zip", "tar", "rar", "bz2", "gz", "xz"].includes(ext)) return "bi-file-zip-fill";
		if (["ics"].includes(ext)) return "bi-calendar-event";
		if (a.ContentType.match(/^audio\//)) return "bi-file-music-fill";
		if (a.ContentType.match(/^video\//)) return "bi-file-play-fill";
		if (a.ContentType.match(/\/calendar$/)) return "bi-file-check-fill";
		if (a.ContentType.match(/^text\//) || ["txt", "sh", "log"].includes(ext)) return "bi-file-text-fill";
		return "bi-file-arrow-down-fill";
	}

	function colorHash(s) {
		if (tagColorCache[s] !== undefined) return tagColorCache[s];
		tagColorCache[s] = _colorHash.hex(s);
		return tagColorCache[s];
	}

	function copyToClipboard(text) {
		navigator.clipboard.writeText(text).then(
			() => {
				copiedText.value[text] = true;
				setTimeout(() => {
					delete copiedText.value[text];
				}, 2000);
			},
			() => {
				alert("Failed to copy to clipboard");
			},
		);
	}

	return {
		loading,
		copiedText,
		copyToClipboardSupported,
		resolve,
		searchURI,
		getFileSize,
		formatNumber,
		messageDate,
		secondsToRelative,
		tagEncodeURI,
		getSearch,
		getPaginationParams,
		modal,
		hideNav,
		get,
		post,
		del,
		put,
		handleError,
		allAttachments,
		isImage,
		attachmentIcon,
		colorHash,
		copyToClipboard,
	};
}
