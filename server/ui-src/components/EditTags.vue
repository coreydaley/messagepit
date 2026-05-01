<script setup>
import { ref, watch } from "vue";
import { mailbox } from "../stores/mailbox";
import { useCommon } from "../composables/useCommon";

const { del, put, resolve } = useCommon();

const editableTags = ref([]);
const tagToDelete = ref(false);

watch(
	() => mailbox.tags,
	(tags) => {
		editableTags.value = [];
		tags.forEach((t) => {
			editableTags.value.push({ before: t, after: t });
		});
	},
	{ deep: true },
);

function validTag(t) {
	if (!t.after.match(/^([a-zA-Z0-9\- ._@]){1,100}$/)) {
		return false;
	}

	const lower = t.after.toLowerCase();
	for (let x = 0; x < editableTags.value.length; x++) {
		if (editableTags.value[x].before !== t.before && lower === editableTags.value[x].before.toLowerCase()) {
			return false;
		}
	}

	return true;
}

function renameTag(t) {
	if (!validTag(t) || t.before === t.after) {
		return;
	}

	put(resolve(`/api/v1/tags/` + encodeURI(t.before)), { Name: t.after }, () => {
		// the API triggers a reload via websockets
	});
}

function deleteTag() {
	del(resolve(`/api/v1/tags/` + encodeURI(tagToDelete.value.before)), null, () => {
		// the API triggers a reload via websockets
		tagToDelete.value = false;
	});
}

function resetTagEdit(t) {
	for (let x = 0; x < editableTags.value.length; x++) {
		if (editableTags.value[x].before !== t.before && editableTags.value[x].before !== editableTags.value[x].after) {
			editableTags.value[x].after = editableTags.value[x].before;
		}
	}
}
</script>

<template>
	<div
		id="EditTagsModal"
		class="modal fade"
		tabindex="-1"
		aria-labelledby="EditTagsModalLabel"
		aria-hidden="true"
		data-bs-keyboard="false"
	>
		<div class="modal-dialog modal-lg">
			<div class="modal-content">
				<div class="modal-header">
					<h5 id="EditTagsModalLabel" class="modal-title">Edit tags</h5>
					<button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
				</div>
				<div class="modal-body">
					<p>
						Renaming a tag will update the tag for all messages. Deleting a tag will only delete the tag
						itself, and not any messages which had the tag.
					</p>
					<div v-for="(t, i) in editableTags" :key="'tag_' + i" class="mb-3">
						<div class="input-group has-validation">
							<input
								v-model.trim="t.after"
								type="text"
								class="form-control"
								:class="!validTag(t) ? 'is-invalid' : ''"
								aria-describedby="inputGroupPrepend"
								required
								@keydown.enter="renameTag(t)"
								@keydown.esc="t.after = t.before"
								@focus="resetTagEdit(t)"
							/>
							<button v-if="t.before != t.after" class="btn btn-success" @click="renameTag(t)">
								Save
							</button>
							<template v-else>
								<button
									class="btn btn-outline-danger"
									:class="tagToDelete.before == t.before ? 'text-white btn-danger' : ''"
									@click="!tagToDelete ? (tagToDelete = t) : deleteTag()"
									@blur="tagToDelete = false"
								>
									<template v-if="tagToDelete == t"> Confirm? </template>
									<template v-else> Delete </template>
								</button>
							</template>
							<div class="invalid-feedback">Invalid tag name</div>
						</div>
					</div>
				</div>
				<div class="modal-footer">
					<button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">Close</button>
				</div>
			</div>
		</div>
	</div>
</template>
