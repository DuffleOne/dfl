<template>
	<div class="container mx-auto">
		<div class="card shadow-lg p-6 mb-6">
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
					<input type="text" placeholder="username" class="w-full input input-primary input-bordered" v-model.trim="localState.username">
					<button class="absolute top-0 right-0 rounded-l-none btn btn-primary text-neutral-content" v-on:click.prevent="login()">Login</button>
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
	username: '',
	clientId: '',
	codeChallenge: '',
	codeChallengeMethod: '',
	nonce: '',
	responseType: '',
	scopes: [],
	state: '',
	client: null,
})

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

}
</script>
