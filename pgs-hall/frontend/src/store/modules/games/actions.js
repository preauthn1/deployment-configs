import { MD5 } from "crypto-js";
import { instanceHeaders , getGlobalSettings , setDocumentTitle  } from "@/utilities/mixin/commonMethods";

export default {

    async loadBalanceAndSettings({ commit, rootGetters }) {
       
        let url = new URL(process.env.VUE_APP_API_URL + "v1/lobby/settings");
        let dateTime = Date.now();
        let token = [REDACTED]"user/token"];
        let playerName = rootGetters["user/playerName"];
        let signKey = MD5(`${getGlobalSettings().encrypt_salt_key}_${dateTime}_${playerName}`).toString();

        let params = {
            signKey: signKey,
            ts: dateTime,
            playerName: playerName,
        };
      
        url.search = new URLSearchParams(params).toString();

        let response = await fetch(url, {
            method: "GET",
            headers: instanceHeaders(token),
        });
        let responseData = await response.json();

        if (!response.ok) {
            const error = new Error(responseData.message);
            commit('setBalance', -1);
            throw (error);
        }
        
        const lobbySetting = 
        {
            isIframeEnable: responseData.data.isIframeEnable,
            isOpenNewTab : responseData.data.isOpenNewTab,
            externalExitLink: responseData.data.externalExitLink,
            isAllGameMaintain: responseData.data.isAllGameMaintain,
            maintainMsg: responseData.data.maintainMsg,
            lobbyStyle : responseData.data.lobbyStyle
        };

        commit('setLobbySetting', lobbySetting);        
        commit('setBalance', responseData.data.balance);
        commit('user/setPlayerName', responseData.data.playerName, { root: true });     

        //修改title名稱
        if( responseData.data.siteDisplayName) {
            setDocumentTitle(responseData.data.siteDisplayName);
        }
    },


    async loadGames({ commit, rootGetters }) {

        //2021-10-13 新增，指定查詢特定遊戲商維護訊息 (如果有queryString)
        /*
        const urlParams = new URLSearchParams(window.location.search);
        let targetMaintainVendorID = urlParams.get('targetMaintainVendorID');

        if(!targetMaintainVendorID || targetMaintainVendorID == null)
        {
            targetMaintainVendorID = '';
        }

        let url = new URL(process.env.VUE_APP_API_URL + "v1/lobby/games");
        let dateTime = Date.now();
        let token = [REDACTED]"user/token"];
        let playerName = rootGetters["user/playerName"];
        let signKey = MD5(`${getGlobalSettings().encrypt_salt_key}_${dateTime}_${playerName}`).toString();
        let params = {
            signKey: signKey,
            ts: dateTime,
            playerName: playerName,
            targetMaintainVendorID : targetMaintainVendorID
        };
        */
        let url = new URL(location.origin + "/games");
        const lang = localStorage.getItem("LOCALE");
        let params = { lang };
      
        url.search = new URLSearchParams(params).toString();
        let response = await fetch(url, {
            method: "GET",
            /*
            headers: instanceHeaders(token),
            */
        });

        let isFetchError = false;

        let responseData = await response.json()
            .catch((x)=>
            {            
                console.log(x);
                console.log('catch error 2');
                isFetchError = true;
            });

        if(responseData === undefined && isFetchError)
        {
            throw new Error('Fetch game list fail. The possible reason is that the system is currently under maintenance. 获取游戏清单失败。可能原因为系统目前维护中');
        }

        if (!response.ok) {
            const error = new Error(responseData.message);
            commit('setBalance', -1);
            throw (error);
        }

        /*
        let games = [];
        for (const game of responseData.data.games) {
            games.push(game);
        }
        commit('setGameTypes',responseData.data.gameTypes);
        */

        //後端吐的 -99 代表所有遊戲
        let games = responseData
            .filter(g => g.gameType === -99)
            .map(item=> 
            {        
                return item.games;
            }).flat();

        // for fishing king
        games.forEach(game => {
          if (game.gameID == 310001) {
              game.gameName = "Fishing King";
              game.imageUrl = "https://example.invalid/game-icon/aggfish_fishingking.png";
          }
        })

        // for Apollo
        /*
        let apolloIndex = games.findIndex(game => game.gameID === 290088);
        if (apolloIndex !== -1) {
            let gameObj = games.splice(apolloIndex, 1)[0];
            games.unshift(gameObj);
        }
        */

        const gameTypes = responseData
        .filter(g => g.gameType !== -99)
        .map(item=> {
          return {
            gameTypeName: item.gameTypeName,
            gameType: item.gameType,
          }
        });
        commit('setGameTypes', gameTypes);
        /*
        if(responseData.data.isAllGameMaintain) {
            games = [];
        }        
      
        */
        commit('setGames', games);
        /*
        commit('setRecentlyGames', responseData.data.recentlyGames);
        commit('setBalance', responseData.data.balance);
        commit('user/setPlayerName', responseData.data.playerName, { root: true });     

        //修改title名稱
        if( responseData.data.siteDisplayName) {
            setDocumentTitle(responseData.data.siteDisplayName);
        }
        */
    },

    async fetchGameLink({ rootGetters }, data) {
        let url = new URL(process.env.VUE_APP_API_URL + "v1/lobby/game-link");
        let dateTime = Date.now();
        let token = [REDACTED]"user/token"];
        let playerName = rootGetters["user/playerName"];
        let signKey = MD5(`${getGlobalSettings().encrypt_salt_key}_${dateTime}_${playerName}`).toString();
        let params = {
            signKey: signKey,
            ts: dateTime,
            playerName: playerName,
            gameID: data.gameID,
            checkCode : data.checkCode
        };
        url.search = new URLSearchParams(params).toString();
        let response = await fetch(url, {
            method: "GET",
            headers: instanceHeaders(token),
        });
        let responseData = await response.json();
        
        if (!response.ok) {
            const error = new Error(responseData.message);
            throw (error);
        }

        //修改title名稱
        if(responseData.data.gameName) {
            const newTitle =  responseData.data.gameName + ' - ' + responseData.data.siteDisplayName;            
            setDocumentTitle(newTitle);
        }

        return responseData.data;
    },

    async fetchIframeGameLink({ rootGetters }, data) {
        let url = new URL(process.env.VUE_APP_API_URL + "v1/iframe/game-link");
        let dateTime = Date.now();
        let token = [REDACTED]"user/token"];
        let playerName = rootGetters["user/playerName"];
        let signKey = MD5(`${getGlobalSettings().encrypt_salt_key}_${dateTime}_${playerName}`).toString();
        let params = {
            signKey: signKey,
            ts: dateTime,
            playerName: playerName,
            gameID: data.gameID,
            checkCode: data.checkCode
        };
        url.search = new URLSearchParams(params).toString();
        let response = await fetch(url, {
            method: "GET",
            headers: instanceHeaders(token),
        });
        let responseData = await response.json();
        if (!response.ok) {
            const error = new Error(responseData.message);
            throw (error);
        }

        //修改title名稱
        if(responseData.data.gameName) {
            const newTitle =  responseData.data.gameName + ' - ' + responseData.data.siteDisplayName;            
            setDocumentTitle(newTitle);
        }

        return responseData.data;
    },
    async fetchEventInfo({ rootGetters }) {
        let url = new URL(process.env.VUE_APP_API_URL + "v1/events/event-info");
        let token = [REDACTED]"user/token"];
        let response = await fetch(url, {
            method: "GET",
            headers: instanceHeaders(token),
        });
        let responseData = await response.json();
        if (!response.ok) {
            const error = new Error(responseData.message);
            throw (error);
        }
        return responseData.data;
    },

    async eventsLuckyDrawRedEnvelope({ rootGetters }) {
        let url = new URL(process.env.VUE_APP_API_URL + "v1/events/lucky-draw/redEnvelope");
        let token = [REDACTED]"user/token"];
        let dateTime = Date.now();
        let signKey = MD5(`${getGlobalSettings().encrypt_salt_key}_${dateTime}`).toString();
        let params = {
            signKey: signKey,
            ts: dateTime,
        };
        let response = await fetch(url, {
            method: "POST",
            headers: instanceHeaders(token),
            body: JSON.stringify(params),
        });

        let responseData = await response.json();
        if (!response.ok) {
            const error = new Error(responseData.message);
            throw (error);
        }
        return responseData.data;
    },
};
