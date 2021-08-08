<template>
	<div class="container mx-auto">
		<div class="card shadow-lg p-6 sm:w-8/12 mx-auto">
			<div class="form-control">
				<label class="label">
					<span class="label-text">Username</span>
				</label>
				<input type="text" placeholder="username" class="w-full input input-primary input-bordered" v-model="localState.username" required="required">
			</div>
			<div class="form-control">
				<label class="label">
					<span class="label-text">Invite code</span>
				</label>
				<input type="text" placeholder="invite code" class="w-full input input-primary input-bordered" v-model="localState.inviteCode" required="required">
			</div>
			<div class="form-control">
				<label class="label">
					<span class="label-text">Key name</span>
				</label>
				<input type="text" placeholder="key name" class="w-full input input-primary input-bordered" v-model="localState.keyName" required="required">
			</div>
			<div class="form-control">
				<button
					class="btn btn-primary text-neutral-content btn-sm mt-4"
					:class="{ 'btn-disabled': localState.pageDisabled }"
					:disabled="localState.pageDisabled"
					v-on:click.prevent="register"
				>Register</button>
			</div>
		</div>
	</div>
</template>

<script setup>
import { reactive } from 'vue';
import { useStore } from 'vuex';

const store = useStore()

const localState = reactive({
	pageDisabled: false,
	username: '',
	inviteCode: '',
	keyName: '',
});

if (!window.PublicKeyCredential) {
	localState.pageDisabled = true;
	store.commit('error', 'Cannot login without WebAuthn support');
}

const { auth } = store.state.services;

async function register() {
	store.commit('error', null);
	store.commit('success', null);

	const { username, inviteCode, keyName } = localState;

	if (!username || !inviteCode || !keyName) {
		store.commit('error', 'All three fields are required.');
		return;
	}

	const prompt = await registerPrompt(username, inviteCode);

	const challengeId = prompt?.id;
	const { publicKey } = prompt?.challenge;

	publicKey.challenge = bufferDecode(publicKey.challenge);
	publicKey.user.id = bufferDecode(publicKey.user.id);

	for (const i in publicKey.excludeCredentials)
		publicKey.excludeCredentials[i].id = bufferDecode(publicKey.excludeCredentials[i].id);

	const credential = await handleCredential(publicKey);

	console.log(credential);

	try {
		await auth('/1/2021-01-15/register_confirm', {
			username,
			inviteCode,
			challengeId,
			keyName: keyName === '' ? null : keyName,
			webauthn: {
				id: credential.id,
				rawId: bufferEncode(credential.rawId),
				type: credential.type,
				response: {
					attestationObject: bufferEncode(credential.response.attestationObject),
					clientDataJson: bufferEncode(credential.response.clientDataJSON),
				},
			},
		});
	} catch (error) {
		store.commit('error', error?.code);

		throw error;
	}

	localState.inviteCode = '';

	store.commit('success', 'Successfully registered.');
}

async function registerPrompt(username, inviteCode) {
	try {
		return await auth('1/2021-01-15/register_prompt', { username, inviteCode });
	} catch (error) {
		store.commit('error', error?.code);

		throw error;
	}
}

async function handleCredential(publicKey) {
	try {
		return await navigator.credentials.create({ publicKey });
	} catch (error) {
		// TODO(lm): This seems awful
		if (String(error).includes('excludeCredentials')) {
			store.commit('error', 'This key has already been registered');
		}

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
