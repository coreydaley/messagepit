<script>
import CommonMixins from "../mixins/CommonMixins";
import { mailbox } from "../stores/mailbox";
import { limitOptions, pagination } from "../stores/pagination";

export default {
	mixins: [CommonMixins],

	props: {
		total: {
			type: Number,
			default: 0,
		},
		count: {
			type: Number,
			default: null,
		},
	},

	data() {
		return {
			pagination,
			mailbox,
			limitOptions,
		};
	},

	computed: {
		canPrev() {
			return pagination.start > 0;
		},

		canNext() {
			const c = this.count !== null ? this.count : mailbox.messages.length;
			return this.total > pagination.start + c;
		},

		// returns the number of next X messages
		nextMessages() {
			let t = pagination.start + parseInt(pagination.limit, 10);
			if (t > this.total) {
				t = this.total;
			}

			return t;
		},
	},

	methods: {
		changeLimit() {
			pagination.start = 0;
			this.updateQueryParams();
		},

		viewNext() {
			pagination.start = parseInt(pagination.start, 10) + parseInt(pagination.limit, 10);
			this.updateQueryParams();
		},

		viewPrev() {
			let s = pagination.start - pagination.limit;
			if (s < 0) {
				s = 0;
			}
			pagination.start = s;
			this.updateQueryParams();
		},

		updateQueryParams() {
			const path = this.$route.path;
			const p = {
				...this.$route.query,
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
			this.$router.push(path + "?" + params.toString());
		},
	},
};
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
