import { reactive } from "vue";

export const webhooksStore = reactive({
	total: 0,
	unread: 0,
	messages: [],
	selected: null,
});
