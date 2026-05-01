<script setup>
import AjaxLoader from "../AjaxLoader.vue";
import Tags from "bootstrap5-tags";
import { ref, onMounted } from "vue";
import { mailbox } from "../../stores/mailbox";
import { useCommon } from "../../composables/useCommon";

const props = defineProps({
	message: {
		type: Object,
		default: () => ({}),
	},
});

const emit = defineEmits(["delete"]);

const { loading, post, resolve, modal } = useCommon();

const addresses = ref([]);
const deleteAfterRelease = ref(false);
const allAddresses = ref([]);

function releaseMessage() {
	window.setTimeout(() => {
		if (!addresses.value.length) {
			return false;
		}

		const data = {
			To: addresses.value,
		};

		post(resolve("/api/v1/message/" + props.message.ID + "/release"), data, () => {
			modal("ReleaseModal").hide();
			if (deleteAfterRelease.value) {
				emit("delete");
			}
		});
	}, 100);
}

onMounted(() => {
	const a = [];
	for (const i in props.message.To) {
		a.push(props.message.To[i].Address);
	}
	for (const i in props.message.Cc) {
		a.push(props.message.Cc[i].Address);
	}
	for (const i in props.message.Bcc) {
		a.push(props.message.Bcc[i].Address);
	}

	allAddresses.value = JSON.parse(JSON.stringify([...new Map(a.map((ad) => [ad.toLowerCase(), ad])).values()]));

	const defaultAddr = mailbox.defaultReleaseAddresses;
	for (const i in defaultAddr) {
		if (!allAddresses.value.includes(defaultAddr[i])) {
			allAddresses.value.push(defaultAddr[i]);
		}
	}

	if (defaultAddr.length === 0) {
		addresses.value = allAddresses.value;
	} else {
		addresses.value = defaultAddr;
	}

	Tags.init("select[multiple]");
});
</script>

<template>
	<div id="ReleaseModal" class="modal fade" tabindex="-1" aria-labelledby="AppInfoModalLabel" aria-hidden="true">
		<div v-if="message" class="modal-dialog modal-xl">
			<div class="modal-content">
				<div class="modal-header">
					<h1 id="AppInfoModalLabel" class="modal-title fs-5">Release email</h1>
					<button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
				</div>
				<div class="modal-body">
					<h6>Send this message to one or more addresses specified below.</h6>
					<div class="row">
						<label class="col-sm-2 col-form-label text-body-secondary">From</label>
						<div class="col-sm-10">
							<input
								v-if="mailbox.uiConfig.MessageRelay.OverrideFrom != ''"
								type="text"
								aria-label="From address"
								readonly
								class="form-control-plaintext"
								:value="mailbox.uiConfig.MessageRelay.OverrideFrom"
							/>
							<input
								v-else
								type="text"
								aria-label="From address"
								readonly
								class="form-control-plaintext"
								:value="message.From ? message.From.Address : ''"
							/>
						</div>
					</div>
					<div class="row">
						<label class="col-sm-2 col-form-label text-body-secondary">Subject</label>
						<div class="col-sm-10">
							<input
								type="text"
								aria-label="Subject"
								readonly
								class="form-control-plaintext"
								:value="message.Subject"
							/>
						</div>
					</div>
					<div class="row mb-3">
						<label class="col-sm-2 col-form-label text-body-secondary">Send to</label>
						<div class="col-sm-10">
							<select
								v-model="addresses"
								class="form-select tag-selector"
								multiple
								data-allow-new="true"
								data-clear-end="true"
								data-allow-clear="true"
								data-placeholder="Enter email addresses..."
								data-add-on-blur="true"
								data-badge-style="primary"
								data-regex='^(([^<>()[\]\\.,;:\s@"]+(\.[^<>()[\]\\.,;:\s@"]+)*)|.(".+"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\])|(([a-zA-Z\-0-9]+\.)+[a-zA-Z]{2,}))$'
								data-separator="|,|"
							>
								<option value="">Enter email addresses...</option>
								<!-- you need at least one option with the placeholder -->
								<option v-for="t in allAddresses" :key="'address+' + t" :value="t">{{ t }}</option>
							</select>
							<div class="invalid-feedback">Invalid email address</div>
							<div class="form-text mt-1">
								Default release addresses can be configured in
								<a href="#" data-bs-toggle="modal" data-bs-target="#SettingsModal">
									<i class="bi bi-gear-fill ms-1"></i>
									Settings </a
								>.
							</div>
						</div>
					</div>
					<div class="row mb-3">
						<div class="col-sm-10 offset-sm-2">
							<div class="form-check">
								<input
									id="DeleteAfterRelease"
									v-model="deleteAfterRelease"
									class="form-check-input"
									type="checkbox"
								/>
								<label class="form-check-label" for="DeleteAfterRelease">
									Delete the message after release
								</label>
							</div>
						</div>
					</div>

					<h6>Notes</h6>
					<ul>
						<li v-if="mailbox.uiConfig.MessageRelay.AllowedRecipients != ''" class="form-text">
							A recipient <b>allowlist</b> has been configured. Any mail address not matching the
							following will be rejected:
							<code>{{ mailbox.uiConfig.MessageRelay.AllowedRecipients }}</code>
						</li>
						<li v-if="mailbox.uiConfig.MessageRelay.BlockedRecipients != ''" class="form-text">
							A recipient <b>blocklist</b> has been configured. Any mail address matching the following
							will be rejected:
							<code>{{ mailbox.uiConfig.MessageRelay.BlockedRecipients }}</code>
						</li>
						<li v-if="!mailbox.uiConfig.MessageRelay.PreserveMessageIDs" class="form-text">
							For testing purposes, a new unique <code>Message-ID</code> will be generated on send.
						</li>
						<li v-if="mailbox.uiConfig.MessageRelay.OverrideFrom != ''" class="form-text">
							The <code>From</code> email address has been overridden by the relay configuration to
							<code>{{ mailbox.uiConfig.MessageRelay.OverrideFrom }}</code
							>.
						</li>
						<li class="form-text">
							SMTP delivery failures will bounce back to
							<code v-if="mailbox.uiConfig.MessageRelay.ReturnPath != ''">
								{{ mailbox.uiConfig.MessageRelay.ReturnPath }}
							</code>
							<code v-else-if="mailbox.uiConfig.MessageRelay.OverrideFrom != ''">
								{{ mailbox.uiConfig.MessageRelay.OverrideFrom }}
							</code>
							<code v-else>{{ message.ReturnPath }}</code
							>.
						</li>
					</ul>
				</div>
				<div class="modal-footer">
					<button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">Cancel</button>
					<button type="button" class="btn btn-primary" :disabled="!addresses.length" @click="releaseMessage">
						Release
					</button>
				</div>
			</div>
		</div>
	</div>

	<AjaxLoader :loading="loading" />
</template>
