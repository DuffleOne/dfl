export default {
	error(state, message) {
		state.successMessage = null;
		state.errorMessage = message;
	},

	success(state, message) {
		state.errorMessage = null;
		state.successMessage = message;
	},
};
