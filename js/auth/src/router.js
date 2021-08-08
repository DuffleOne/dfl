import HomePage from './components/HomePage.vue';
import ManagePage from './components/ManagePage.vue';
import RegisterPage from './components/RegisterPage.vue';
import { createRouter, createWebHashHistory } from 'vue-router';

const routes = [
	{ path: '/', component: HomePage },
	{ path: '/register', component: RegisterPage },
	{ path: '/manage', component: ManagePage },
];

export default createRouter({
	history: createWebHashHistory(),
	routes,
});
