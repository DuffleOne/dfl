<template>
	<div class="container mx-auto">
		<div class="navbar mb-2 shadow-lg bg-neutral text-neutral-content sm:rounded-box sm:my-3">
			<div class="px-2 mx-2 navbar-start">
				<span class="text-lg font-bold">DFL</span>
			</div>
			<div class="hidden px-2 mx-2 navbar-center lg:flex">
				<div class="flex items-stretch">
					<router-link class="btn btn-ghost btn-sm rounded-btn" to="/">Home</router-link>
					<router-link class="btn btn-ghost btn-sm rounded-btn" to="/register">Register</router-link>
					<router-link class="btn btn-ghost btn-sm rounded-btn" to="/manage">Manage</router-link>
				</div>
			</div>
			<div class="navbar-end">
				<a href="https://github.com/DuffleOne">
					<div class="avatar">
						<div class="rounded-full w-10 h-10 m-1">
							<img src="https://avatars.githubusercontent.com/u/1222340?v=4">
						</div>
					</div>
				</a>
			</div>
		</div>
	</div>
</template>
