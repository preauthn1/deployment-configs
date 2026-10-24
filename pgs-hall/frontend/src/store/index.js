import { createStore } from 'vuex';
import gamesModule from './modules/games/index.js';
import userModule from './modules/user/index.js';
import createPersistedState from 'vuex-persistedstate'

const store = createStore({
    plugins: [createPersistedState({
        storage: window.sessionStorage,
        paths: ['user'],
    })],
    modules: {
        games: gamesModule,
        user: userModule,
    },
});

export default store;