<template>
	<div class="container mx-auto">
		<div v-if="localState.display" class="card shadow-lg mb-6">
			<div class="p-6">
				<div class="prose">
					<h1>Authenticated!</h1>
					<div class="relative">
						<h3><code id="code_value">{{ localState.display.authorizationCode }}</code></h3>
						<a class="copyBtn btn btn-sm btn-outline absolute top-0 right-0" data-clipboard-target="#code_value"><i class="far fa-clipboard"></i></a>
					</div>
					<p>Be careful to copy this code exactly into your application.</p>
				</div>
			</div>
			<div class="overflow-x-auto">
				<table class="table table-compact w-full">
					<thead>
						<th scope="col">Attribute</th>
						<th scope="col">Value</th>
					</thead>
					<tbody>
						<tr>
							<th scope="row">State</th>
							<td><code>{{ localState.state }}</code></td>
						</tr>
						<tr>
							<th scope="row">Expires in</th>
							<td><code>{{ localState.display.expiresIn }}</code></td>
						</tr>
						<tr>
							<th scope="row">Expires at</th>
							<td><code>{{ localState.display.expiresAt }}</code></td>
						</tr>
					</tbody>
				</table>
			</div>
		</div>
		<div v-if="!localState.display" class="card shadow-lg p-6 mb-6">
			<div class="prose">
				<h1>Authenticate</h1>
				<p>Login to <code>{{ localState.client?.name }}</code></p>
				<p>The app is requesting the following scopes:</p>
			</div>
			<div class="flex flex-nowrap justify-start mb-6">
				<div v-for="scope in localState.scopes" class="flex-shrink card card-side shadow-lg text-center bg-neutral text-neutral-content mx-1">
					<div class="card-body p-2 prose">
						<span class="card-title">{{ scope }}</span>
					</div>
				</div>
			</div>
			<div class="form-control">
				<label class="label">
					<span class="label-text">Username</span>
				</label>
				<div class="relative">
					<input
						type="text"
						placeholder="username"
						class="w-full input input-primary input-bordered"
						v-model.trim="localState.username"
						v-on:keyup.enter="login()"
					>
					<button
						class="absolute top-0 right-0 rounded-l-none btn btn-primary text-neutral-content"
						:class="{ 'btn-disabled': localState.disabled }"
						:disabled="localState.disabled"
						v-on:click.prevent="login()"
					>Login</button>
				</div>
			</div>
		</div>
	</div>
</template>

<script setup>
import { useStore } from 'vuex';
import { reactive, onMounted } from 'vue';

const store = useStore();
const { auth } = store.state.services;

const localState = reactive({
	disabled: false,
	username: '',
	clientId: '',
	codeChallenge: '',
	codeChallengeMethod: '',
	nonce: '',
	responseType: '',
	scopes: [],
	state: '',
	redirectUri: null,
	client: null,
	display: null,
});

onMounted(async () => {
	const rawParamsStr = decodeURI(window.location.hash.split('?')[1]);
	const params = rawParamsStr.split('&');

	for (const param of params) {
		const [key, value] = param.split('=');

		switch (key) {
			// TODO(lm): just make this dynamically adjust case
			case 'client_id':
				localState.clientId = value;
				break;
			case 'code_challenge':
				localState.codeChallenge = value;
				break;
			case 'code_challenge_method':
				localState.codeChallengeMethod = value;
				break;
			case 'nonce':
				localState.nonce = value;
				break;
			case 'response_type':
				localState.responseType = value;
				break;
			case 'scope':
				localState.scopes = decodeURIComponent(value).split('+');
				break;
			case 'state':
				localState.state = value;
				break;
			case 'redirect_uri':
				localState.redirectUri = value;
				break;
			default:
				continue;
		}
	}

	const client = await auth('/1/2021-01-15/get_client', {
		clientId: localState.clientId,
	});

	localState.client = client;
});

async function login() {
	localState.disabled = true;

	const prompt = await loginPrompt();

	const challengeId = prompt?.id;
	const { publicKey } = prompt?.challenge;

	publicKey.challenge = bufferDecode(publicKey.challenge);

	for (const i in publicKey.allowCredentials)
		publicKey.allowCredentials[i].id = bufferDecode(publicKey.allowCredentials[i].id);

	const credential = await handleCredential(publicKey);

	const handler = await confirm(challengeId, credential);

	switch (handler?.type) {
		case 'redirect':
			window.location = handler?.params?.uri;
			break;
		case 'display':
			localState.display = handler?.params;
			break;
		default:
			store.commit('error', `Unknown handler type: ${handler?.type}`);
	}
}

async function loginPrompt() {
	try {
		return await auth('/1/2021-01-15/authorize_prompt', {
			username: localState.username,
		});
	} catch (error) {
		store.commit('error', error?.code);

		throw error;
	}
}

async function handleCredential(publicKey) {
	try {
		return await navigator.credentials.get({ publicKey });
	} catch (error) {
		store.commit('error', error?.code);

		throw error;
	}
}

async function confirm(challengeId, credential) {
	try {
		return await auth('/1/2021-01-15/authorize_confirm', {
			responseType: localState.responseType,
			redirectUri: localState.redirectUri,
			clientId: localState.client.id,
			scope: localState.scopes.join(' '),
			state: localState.state,
			nonce: localState.nonce,
			codeChallenge: localState.codeChallenge,
			codeChallengeMethod: localState.codeChallengeMethod,
			username: localState.username,
			challengeId,
			webauthn: {
				id: credential.id,
				rawId: bufferEncode(credential.rawId),
				type: credential.type,
				response: {
					authenticatorData: bufferEncode(credential.response.authenticatorData),
					clientDataJson: bufferEncode(credential.response.clientDataJSON),
					signature: bufferEncode(credential.response.signature),
					userHandle: bufferEncode(credential.response.userHandle),
				},
			},
		});
	} catch (error) {
		store.commit('error', error?.code);

		throw error;
	}
}

function bufferDecode(value) {
	return Uint8Array.from(atob(value), c => c.charCodeAt(0));
}

function bufferEncode(value) {
	return btoa(String.fromCharCode.apply(null, new Uint8Array(value)))
		.replace(/\+/g, "-")
		.replace(/\//g, "_")
		.replace(/=/g, "");
}
</script>
