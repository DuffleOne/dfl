import crpc from 'crpc';

const auth = crpc(getAuthURL());

export default {
	auth,
};

function getAuthURL() {
	// eslint-disable-next-line no-undef
	if (location.hostname.includes('localhost'))
		return 'http://localhost:3000';

	return 'https://api.duffle.one/1/auth';
}
