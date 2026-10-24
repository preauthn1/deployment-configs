import { createRouter, createWebHistory } from 'vue-router';
import Home from './pages/Home.vue';
import EventDescription from './pages/EventDescription.vue';
// import Game from './pages/Game.vue';
const router = createRouter({
    history: createWebHistory(),
    routes: [

        { path: '/EventDescription', component: EventDescription },
        { path: '/:catchAll(.*)', component: Home },
        
    ],
});

export default router;