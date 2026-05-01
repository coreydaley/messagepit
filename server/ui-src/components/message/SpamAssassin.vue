<script setup>
import { ref, computed, watch, onMounted } from "vue";
import { VcDonut } from "vue-css-donut-chart";
import axios from "axios";
import { useCommon } from "../../composables/useCommon";

const props = defineProps({
	message: {
		type: Object,
		default: () => ({}),
	},
});

const emit = defineEmits(["setSpamScore", "setBadgeStyle"]);

const { resolve } = useCommon();

const error = ref(false);
const check = ref(false);

const graphSections = computed(() => {
	const score = check.value.Score;
	let p = Math.round((score / 5) * 100);
	if (p > 100) {
		p = 100;
	} else if (p < 0) {
		p = 0;
	}

	let c = "#ffc107";
	if (check.value.IsSpam) {
		c = "#dc3545";
	}

	return [
		{
			label: score + " / 5",
			value: p,
			color: c,
		},
	];
});

const scoreColor = computed(() => graphSections.value[0].color);

function badgeStyle(ignorePadding = false) {
	let style = "bg-success";
	if (check.value.Error) {
		style = "bg-warning text-primary";
	} else if (check.value.IsSpam) {
		style = "bg-danger";
	} else if (check.value.Score >= 4) {
		style = "bg-warning text-primary";
	}

	if (!ignorePadding && String(check.value.Score).includes(".")) {
		style += " p-1";
	}

	return style;
}

function setIcons() {
	let score = check.value.Score;
	if (check.value.Error && check.value.Error !== "") {
		score = "!";
	}
	emit("setBadgeStyle", badgeStyle());
	emit("setSpamScore", score);
}

function doCheck() {
	check.value = false;

	axios
		.get(resolve("/api/v1/message/" + props.message.ID + "/sa-check"), null)
		.then((result) => {
			check.value = result.data;
			error.value = false;
			setIcons();
		})
		.catch((err) => {
			if (err.response && err.response.data) {
				if (err.response.data.Error) {
					error.value = err.response.data.Error;
				} else {
					error.value = err.response.data;
				}
			} else if (err.request) {
				error.value = "Error sending data to the server. Please try again.";
			} else {
				error.value = err.message;
			}
		});
}

watch(
	() => props.message,
	() => {
		emit("setSpamScore", false);
		doCheck();
	},
	{ deep: true },
);

onMounted(() => {
	doCheck();
});
</script>

<template>
	<div class="row mb-3 w-100 align-items-center">
		<div class="col">
			<h4 class="mb-0">Spam Analysis</h4>
		</div>
		<div class="col-auto">
			<button class="btn btn-outline-secondary" data-bs-toggle="modal" data-bs-target="#AboutSpamAnalysis">
				<i class="bi bi-info-circle-fill"></i>
				Help
			</button>
		</div>
	</div>

	<template v-if="error || check.Error != ''">
		<p>Your message could not be checked</p>
		<div v-if="error" class="alert alert-warning">
			{{ error }}
		</div>
		<div v-else class="alert alert-warning">
			There was an error contacting the configured SpamAssassin server: {{ check.Error }}
		</div>
	</template>

	<template v-else-if="check">
		<div class="row w-100 mt-5">
			<div class="col-xl-5 mb-2">
				<vc-donut
					:sections="graphSections"
					background="var(--bs-body-bg)"
					:size="230"
					unit="px"
					:thickness="20"
					:total="100"
					:start-angle="270"
					:auto-adjust-text-size="true"
					foreground="#198754"
				>
					<h2 class="m-0" :class="scoreColor">{{ check.Score }} / 5</h2>
					<div class="text-body mt-2">
						<span v-if="check.IsSpam" class="text-white badge rounded-pill bg-danger p-2">Spam</span>
						<span v-else class="badge rounded-pill p-2" :class="badgeStyle()">Not spam</span>
					</div>
				</vc-donut>
			</div>
			<div class="col-xl-7">
				<div class="row w-100 py-2 border-bottom">
					<div class="col-2 col-lg-1">
						<strong>Score</strong>
					</div>
					<div class="col-10 col-lg-5">
						<strong>Rule <span class="d-none d-lg-inline">name</span></strong>
					</div>
					<div class="col-auto d-none d-lg-block">
						<strong>Description</strong>
					</div>
				</div>

				<div v-for="r in check.Rules" :key="'rule_' + r.Name" class="row w-100 py-2 border-bottom small">
					<div class="col-2 col-lg-1">
						{{ r.Score }}
					</div>
					<div class="col-10 col-lg-5">
						{{ r.Name }}
					</div>
					<div class="col-auto col-lg-6 mt-2 mt-lg-0 offset-2 offset-lg-0">
						{{ r.Description }}
					</div>
				</div>
			</div>
		</div>
	</template>

	<div
		id="AboutSpamAnalysis"
		class="modal fade"
		tabindex="-1"
		aria-labelledby="AboutSpamAnalysisLabel"
		aria-hidden="true"
	>
		<div class="modal-dialog modal-lg modal-dialog-scrollable">
			<div class="modal-content">
				<div class="modal-header">
					<h1 id="AboutSpamAnalysisLabel" class="modal-title fs-5">About Spam Analysis</h1>
					<button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
				</div>
				<div class="modal-body">
					<div id="SpamAnalysisAboutAccordion" class="accordion">
						<div class="accordion-item">
							<h2 class="accordion-header">
								<button
									class="accordion-button collapsed"
									type="button"
									data-bs-toggle="collapse"
									data-bs-target="#col1"
									aria-expanded="false"
									aria-controls="col1"
								>
									What is Spam Analysis?
								</button>
							</h2>
							<div
								id="col1"
								class="accordion-collapse collapse"
								data-bs-parent="#SpamAnalysisAboutAccordion"
							>
								<div class="accordion-body">
									<p>
										MessagePit integrates with SpamAssassin to provide you with some insight into
										the "spamminess" of your messages. It sends your complete message (including any
										attachments) to a running SpamAssassin server and then displays the results
										returned by SpamAssassin.
									</p>
								</div>
							</div>
						</div>
						<div class="accordion-item">
							<h2 class="accordion-header">
								<button
									class="accordion-button collapsed"
									type="button"
									data-bs-toggle="collapse"
									data-bs-target="#col2"
									aria-expanded="false"
									aria-controls="col2"
								>
									How does the point system work?
								</button>
							</h2>
							<div
								id="col2"
								class="accordion-collapse collapse"
								data-bs-parent="#SpamAnalysisAboutAccordion"
							>
								<div class="accordion-body">
									<p>
										The default spam threshold is <code>5</code>, meaning any score lower than 5 is
										considered ham (not spam), and any score of 5 or above is spam.
									</p>
									<p>
										SpamAssassin will also return the tests which are triggered by the message.
										These tests can differ depending on the configuration of your SpamAssassin
										server. The total of this score makes up the the "spamminess" of the message.
									</p>
								</div>
							</div>
						</div>
						<div class="accordion-item">
							<h2 class="accordion-header">
								<button
									class="accordion-button collapsed"
									type="button"
									data-bs-toggle="collapse"
									data-bs-target="#col3"
									aria-expanded="false"
									aria-controls="col3"
								>
									But I don't agree with the results...
								</button>
							</h2>
							<div
								id="col3"
								class="accordion-collapse collapse"
								data-bs-parent="#SpamAnalysisAboutAccordion"
							>
								<div class="accordion-body">
									<p>
										MessagePit does not manipulate the results nor determine the "spamminess" of
										your message. The result is what SpamAssassin returns, and it entirely dependent
										on how SpamAssassin is set up and optionally trained.
									</p>
									<p>
										This tool is simply provided as an aid to assist you. If you are running your
										own instance of SpamAssassin, then you look into your SpamAssassin
										configuration.
									</p>
								</div>
							</div>
						</div>
						<div class="accordion-item">
							<h2 class="accordion-header">
								<button
									class="accordion-button collapsed"
									type="button"
									data-bs-toggle="collapse"
									data-bs-target="#col4"
									aria-expanded="false"
									aria-controls="col4"
								>
									Where can I find more information about the triggered rules?
								</button>
							</h2>
							<div
								id="col4"
								class="accordion-collapse collapse"
								data-bs-parent="#SpamAnalysisAboutAccordion"
							>
								<div class="accordion-body">
									<p>
										Unfortunately the current
										<a href="https://spamassassin.apache.org/" target="_blank"
											>SpamAssassin website</a
										>
										no longer contains any relative documentation about these, most likely because
										the rules come from different locations and change often. You will need to
										search the internet for these yourself.
									</p>
								</div>
							</div>
						</div>
					</div>
				</div>
				<div class="modal-footer">
					<button type="button" class="btn btn-secondary" data-bs-dismiss="modal">Close</button>
				</div>
			</div>
		</div>
	</div>
</template>
