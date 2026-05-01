<script setup>
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { mailbox } from "../stores/mailbox";
import { limitOptions, pagination } from "../stores/pagination";
import { useCommon } from "../composables/useCommon";

const props = defineProps({
	total: {
		type: Number,
		default: 0,
	},
	count: {
		type: Number,
		default: null,
	},
});

const route = useRoute();
const router = useRouter();
const { formatNumber } = useCommon();

const canPrev = computed(() => pagination.start > 0);

const canNext = computed(() => {
	const c = props.count !== null ? props.count : mailbox.messages.length;
	return props.total > pagination.start + c;
});

const nextMessages = computed(() => {
	let t = pagination.start + parseInt(pagination.limit, 10);
	if (t > props.total) {
		t = props.total;
	}
	return t;
});

function updateQueryParams() {
	const path = route.path;
	const p = {
		...route.query,
	};
	if (pagination.start > 0) {
		p.start = pagination.start.toString();
	} else {
		delete p.start;
	}
	if (pagination.limit !== pagination.defaultLimit) {
		p.limit = pagination.limit.toString();
	} else {
		delete p.limit;
	}
	const params = new URLSearchParams(p);
	router.push(path + "?" + params.toString());
}

function changeLimit() {
	pagination.start = 0;
	updateQueryParams();
}

function viewNext() {
	pagination.start = parseInt(pagination.start, 10) + parseInt(pagination.limit, 10);
	updateQueryParams();
}

function viewPrev() {
	let s = pagination.start - pagination.limit;
	if (s < 0) {
		s = 0;
	}
	pagination.start = s;
	updateQueryParams();
}
</script>

<template>
	<div class="d-flex align-items-center justify-content-center gap-2 py-2 border-top">
		<button
			class="btn btn-sm btn-outline-secondary"
			:disabled="!canPrev"
			:title="'View previous ' + pagination.limit + ' messages'"
			@click="viewPrev"
		>
			<i class="bi bi-caret-left-fill"></i>
		</button>

		<small class="text-muted">
			<template v-if="total > 0">
				{{ formatNumber(pagination.start + 1) }}–{{ formatNumber(nextMessages) }}
				of
				{{ formatNumber(total) }}
			</template>
			<span v-else>0 of 0</span>
		</small>

		<select
			v-model="pagination.limit"
			class="form-select form-select-sm w-auto"
			:disabled="total == 0"
			title="Messages per page"
			@change="changeLimit"
		>
			<option v-for="option in limitOptions" :key="option" :value="option">{{ option }} / page</option>
		</select>

		<button
			class="btn btn-sm btn-outline-secondary"
			:disabled="!canNext"
			:title="'View next ' + pagination.limit + ' messages'"
			@click="viewNext"
		>
			<i class="bi bi-caret-right-fill"></i>
		</button>
	</div>
</template>
