<script setup>
import { ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { pagination } from "../stores/pagination";

const emit = defineEmits(["loadMessages"]);

const route = useRoute();
const router = useRouter();

const search = ref("");

function searchFromURL() {
	const urlParams = new URLSearchParams(window.location.search);
	search.value = urlParams.get("q") ? urlParams.get("q") : "";
}

function doSearch(e) {
	pagination.start = 0;
	if (search.value === "") {
		router.push("/");
	} else {
		const urlParams = new URLSearchParams(window.location.search);
		const curr = urlParams.get("q");
		if (curr && curr === search.value) {
			pagination.start = 0;
			emit("loadMessages");
		}
		const p = {
			q: search.value,
		};
		if (pagination.start > 0) {
			p.start = pagination.start.toString();
		}
		if (pagination.limit !== pagination.defaultLimit) {
			p.limit = pagination.limit.toString();
		}

		const params = new URLSearchParams(p);
		router.push("/search?" + params.toString());
	}

	e.preventDefault();
}

function resetSearch() {
	search.value = "";
	router.push("/");
}

watch(route, () => {
	searchFromURL();
});

searchFromURL();
</script>

<template>
	<form class="flex-fill" @submit="doSearch">
		<div class="input-group flex-nowrap">
			<div class="ms-md-2 d-flex border bg-body rounded-start flex-fill position-relative">
				<input
					v-model.trim="search"
					type="text"
					class="form-control border-0"
					aria-label="Search"
					placeholder="Search messages"
				/>
				<span v-if="search != ''" class="btn btn-link position-absolute end-0 text-muted" @click="resetSearch">
					<i class="bi bi-x-circle"></i>
				</span>
			</div>
			<button class="btn btn-outline-secondary" type="submit">
				<i class="bi bi-search"></i>
			</button>
		</div>
	</form>
</template>
