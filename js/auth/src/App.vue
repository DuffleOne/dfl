<template>
	<NavBar />
	<SuccessAlert />
	<ErrorAlert />
	<router-view></router-view>
</template>

<script setup>
import NavBar from "./components/NavBar.vue";
import ErrorAlert from './components/ErrorAlert.vue';
import SuccessAlert from './components/SuccessAlert.vue';
</script>
