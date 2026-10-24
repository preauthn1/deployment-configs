import mutations from './mutations.js';
import actions from './actions.js';
import getters from './getters.js';

export default {
    namespaced: true,
    state() {
        return {
            lobbySetting: {
                isAllGameMaintain: false,
                maintainMsg: '',
                lobbyStyle : 0
            },
            recentlyGames: [],
            games: [],
            gameTypes: [],
            balance: 0,
        }
    },
    mutations,
    actions,
    getters,
};