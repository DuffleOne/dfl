import { createStore } from 'vuex';
import mutations from './mutations.js';
import services from './services.js';

const state = {
	services,
	errorMessage: null,
	successMessage: null,
	authToken: null,
};

export default createStore({
	state,
	mutations,
});
