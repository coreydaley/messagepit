import { ref, watch } from "vue";
import { useCommon } from "./useCommon";
import { mailbox } from "../stores/mailbox";
import { pagination } from "../stores/pagination";

export function useMessages() {
	const common = useCommon();
	const apiURI = ref(null);

	watch(
		() => mailbox.refresh,
		(v) => {
			if (v) loadMessages();
			mailbox.refresh = false;
		},
	);

	function reloadMailbox() {
		pagination.start = 0;
		loadMessages();
	}

	function loadMessages() {
		if (!apiURI.value) {
			alert("apiURI not set!");
			return;
		}

		if (!mailbox.autoPaginating) {
			mailbox.autoPaginating = true;
			return;
		}

		const params = {};
		mailbox.selected = [];
		params.limit = pagination.limit;
		if (pagination.start > 0) {
			params.start = pagination.start;
		}

		common.get(apiURI.value, params, (response) => {
			mailbox.total = response.data.total;
			mailbox.unread = response.data.unread;
			mailbox.tags = response.data.tags;
			mailbox.messages = response.data.messages;
			mailbox.count = response.data.messages_count;
			mailbox.messages_unread = response.data.messages_unread;
			pagination.start = response.data.start;

			if (response.data.count === 0 && response.data.start > 0) {
				pagination.start = 0;
				return loadMessages();
			}

			if (mailbox.lastMessage) {
				window.setTimeout(() => {
					const m = document.getElementById(mailbox.lastMessage);
					if (m) {
						m.focus();
						m.scrollIntoView({ block: "center" });
					} else {
						const mp = document.getElementById("message-page");
						if (mp) mp.scrollTop = 0;
					}
					mailbox.lastMessage = false;
				}, 50);
			} else if (!window.scrollInPlace) {
				const mp = document.getElementById("message-page");
				if (mp) mp.scrollTop = 0;
			}

			window.scrollInPlace = false;
		});
	}

	return {
		...common,
		apiURI,
		reloadMailbox,
		loadMessages,
	};
}
