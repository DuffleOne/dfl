import './index.css';
import App from './App.vue';
import { createApp } from 'vue';
import router from './router.js';
import store from './store/index.js';

router.beforeEach((to, from) => {
	store.commit('error', null);
	store.commit('success', null);
});

createApp(App)
	.use(store)
	.use(router)
	.mount('#app');
