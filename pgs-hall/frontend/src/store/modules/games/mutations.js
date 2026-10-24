export default {

    setLobbySetting(state, payload) {
        state.lobbySetting = payload;
    },   
    setRecentlyGames(state, payload) {
        state.recentlyGames = payload;
    },
    setGames(state, payload) {
        state.games = payload;
    },
    setBalance(state, payload) {
        state.balance = payload;
    },
    setGameTypes(state, payload) {
      state.gameTypes = payload;
  },
};