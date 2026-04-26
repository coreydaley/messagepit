import { reactive } from "vue";

export const smsStore = reactive({
	total: 0,
	unread: 0,
	messages: [],
	selected: null, // currently viewed message ID
	connected: false,
});
