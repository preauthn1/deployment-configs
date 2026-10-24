export default {
    token(state) {
        return state.token;
    },
    playerName(state) {
        return state.playerName;
    },
    lang(state) {
        if (state.lang != null && state.lang != "") {
            return state.lang;
        } else {
            return navigator.language;
        }
    },
};
