import { createRouter, createWebHistory } from 'vue-router';
import HomeView from '../views/HomeView.vue';
import Login from '../views/Login.vue';
import Videos from '@/videos/Videos.vue';

const router = createRouter({
	history: createWebHistory(import.meta.env.BASE_URL),
	routes: [
		{
			path: '/',
			name: 'rtHome',
			component: HomeView,
			children: [
				{
					path: "videos",
					name: "rtVideos",
					component: Videos,
				},
			],
		},
		{
			path: '/login',
			name: 'rtLogin',
			component: Login,
		},
		/*
		{
			path: '/about',
			name: 'about',
			// route level code-splitting
			// this generates a separate chunk (About.[hash].js) for this route
			// which is lazy-loaded when the route is visited.
			component: () => import('../views/AboutView.vue')
		}
		*/
	]
})

export default router
