<template>
<div class="container mx-auto">
		<div class="card shadow-lg p-6 mb-6">
			<div class="form-control">
				<label class="label">
					<span class="label-text">Auth token</span>
				</label>
				<input type="text" placeholder="auth token" class="w-full input input-primary input-bordered" v-model.lazy.trim="localState.authToken">
			</div>
		</div>
		<div class="card shadow-lg mb-6" v-if="localState.username && localState.userId">
			<div class="overflow-x-auto">
				<table class="table w-full">
					<thead>
						<tr>
							<th scope="col">Key</th>
							<th scope="col">Value</th>
						</tr>
					</thead>
					<tbody>
						<tr>
							<th scope="row">User ID</th>
							<td><code>{{ localState.userId }}</code></td>
						</tr>
						<tr>
							<th scope="row">Username</th>
							<td>{{ localState.username }}</td>
						</tr>
					</tbody>
				</table>
			</div>
		</div>
		<div class="card shadow-lg" v-if="localState.keys.length >= 1">
			<div class="overflow-x-auto">
				<table class="table w-full">
					<thead>
						<th scope="col">ID</th>
						<th scope="col">Name</th>
						<th scope="col">Actions</th>
					</thead>
					<tbody>
						<tr v-for="key in localState.keys">
							<td><code>{{ key.id }}</code></td>
							<td>{{ key.name }}</td>
							<td>
								<a class="tooltip" data-tip="Sign" href="#" v-show="!key.signedAt" v-on:click.prevent="signKey(key.id)"><i class="fa fa-signature text-warning px-2"></i></a>
								<a class="tooltip" v-bind:data-tip="signedByText(key)" href="#" ><i class="fa fa-info text-info px-2"></i></a>
								<a class="tooltip" data-tip="Delete" href="#" v-on:click.prevent="deleteKey(key.id)"><i class="fa fa-ban text-error px-2"></i></a>
							</td>
						</tr>
					</tbody>
				</table>
			</div>
			<a href="#" class="btn btn-block" v-on:click.prevent="addKey()">Add key</a>
		</div>
	</div>
</template>

<script setup>
import { useStore } from 'vuex';
import { watch, reactive } from 'vue';

const store = useStore();

const localState = reactive({
	authToken: null,
	userId: null,
	username: null,
	keys: [],
});

const { auth } = store.state.services;

if (window.localStorage) {
	localState.authToken = window.localStorage.getItem('dfl_access_token');
	keyUpdated(localState.authToken);
}

watch(() => localState.authToken, keyUpdated);

async function keyUpdated(newVal, oldVal) {
	store.commit('error', null);

	if (window.localStorage) {
		if (!newVal || newVal === '') {
			localState.userId = null;
			localState.username = null;
			localState.keys = [];
			window.localStorage.removeItem('dfl_access_token');
			return;
		}

		window.localStorage.setItem('dfl_access_token', newVal);
	}

	await whoAmI();
	await listKeys();
}

async function whoAmI() {
	const { authToken } = localState;

	try {
		const whoami = await auth('/1/2021-01-15/whoami', null, {
			headers:  { Authorization: `Bearer ${authToken}` },
		});

		localState.userId = whoami.userId;
		localState.username = whoami.username;
	} catch (error) {
		store.commit('error', error?.code);

		throw error;
	}
}

async function listKeys() {
	const { userId, authToken } = localState;

	try {
		const keys = await auth('/1/2021-01-15/list_u2f_keys', {
			userId,
			includeUnsigned: true,
		}, {
			headers:  { Authorization: `Bearer ${authToken}` },
		});

		localState.keys = keys;
	} catch (error) {
		store.commit('error', error?.code);

		throw error;
	}
}

async function deleteKey(keyId) {
	const userId = localState.userId;

	if (!window.confirm("Are you sure?")) {
		return;
	}

	try {
		await auth('/1/2021-01-15/delete_key', {
			userId,
			keyId,
		}, {
			headers: { Authorization: `Bearer ${localState.authToken}` },
		});

		store.commit('success', 'Key deleted');
	} catch (error) {
		store.commit('error', error?.code);

		throw error;
	}

	await listKeys();
}

async function signKey(keyId) {
	const { userId, authToken } = localState;

	let prompt;

	try {
		prompt = await auth('/1/2021-01-15/sign_key_prompt', {
			userId,
			keyToSign: keyId,
		}, {
			headers: { Authorization: `Bearer ${authToken}` },
		});
	} catch (error) {
		store.commit('error', error?.code);

		throw error;
	}

	const challengeId = prompt?.id;
	const { publicKey } = prompt?.challenge;

	publicKey.challenge = bufferDecode(publicKey.challenge);

	for (const i in publicKey.allowCredentials)
		publicKey.allowCredentials[i].id = bufferDecode(publicKey.allowCredentials[i].id);

	const credential = await navigator.credentials.get({ publicKey });

	try {
		await auth('/1/2021-01-15/sign_key_confirm', {
			userId,
			challengeId,
			keyToSign: keyId,
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
		}, {
			headers: { Authorization: `Bearer ${authToken}` },
		});
	} catch (error) {
		store.commit('error', error?.code);

		throw error;
	}

	store.commit('success', 'Key signed');

	await listKeys();
}

async function addKey() {
	const { userId, authToken } = localState;

	let prompt;

	try {
		prompt = await auth('/1/2021-01-15/create_key_prompt', {
			userId,
		}, {
			headers: { Authorization: `Bearer ${authToken}` },
		});
	} catch (error) {
		store.commit('error', error?.code);

		throw error;
	}

	const keyName = window.prompt('Enter a name for the key');

	const challengeId = prompt?.id;
	const { publicKey } = prompt?.challenge;

	publicKey.challenge = bufferDecode(publicKey.challenge);
	publicKey.user.id = bufferDecode(publicKey.user.id);

	for (const i in publicKey.excludeCredentials)
		publicKey.excludeCredentials[i].id = bufferDecode(publicKey.excludeCredentials[i].id);

	const credential = await handleCredentialRegister(publicKey);

	try {
		await auth('/1/2021-01-15/create_key_confirm', {
			userId,
			challengeId,
			keyName,
			webauthn: {
				id: credential.id,
				rawId: bufferEncode(credential.rawId),
				type: credential.type,
				response: {
					attestationObject: bufferEncode(credential.response.attestationObject),
					clientDataJson: bufferEncode(credential.response.clientDataJSON),
				},
			},
		}, {
			headers: { Authorization: `Bearer ${authToken}` },
		});
	} catch(error) {
		store.commit('error', error?.code);

		throw error;
	}

	await listKeys();
}

async function handleCredentialRegister(publicKey) {
	try {
		return await navigator.credentials.create({ publicKey });
	} catch (error) {
		// TODO(lm): This seems awful
		if (String(error).includes('excludeCredentials')) {
			this.error = 'This key has already been registered';
		}

		throw error;
	}
}

function signedByText(key) {
	if (!key.signedAt)
		return 'Unsigned';

	return `Signed on ${key.signedAt}`;
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
