import { MD5 } from "crypto-js";
import { getGlobalSettings  } from "@/utilities/mixin/commonMethods";

let announcementApi = {


    //未讀彈窗公告
    async getUnreadAnnouncement (vue_instance) { 

        let url = new URL(process.env.VUE_APP_API_URL + "v1/message/announcement");
        let dateTime = Date.now();

        let apiKey = [REDACTED];
        let signKey = MD5(`${apiKey}_${dateTime}`).toString();

        let params = {
            signKey: signKey,
            ts: dateTime,
        };

        url.search = new URLSearchParams(params).toString();

        let response = fetch(url, {
            method: "GET",
            headers: vue_instance.instanceRequestHeaders(),
        }).then((res)=>
        {
            return res.json();
        })
        .catch((error)=>
        {
            console.log(error);
            return error;
        })
        .finally(() =>
        {
        });

        return response;
    },


    //top 20 筆訊息 (玩家個人)
    async getMessageList (vue_instance) { 

        let url = new URL(process.env.VUE_APP_API_URL + "v1/message/player-messages");
        let dateTime = Date.now();

        let apiKey = [REDACTED];
        let signKey = MD5(`${apiKey}_${dateTime}`).toString();

        let params = {
            signKey: signKey,
            ts: dateTime,
        };

        url.search = new URLSearchParams(params).toString();

        let response = fetch(url, {
            method: "GET",
            headers: vue_instance.instanceRequestHeaders(),
        }).then((res)=>
        {
            return res.json();
        })
        .catch((error)=>
        {
            console.log(error);
            return error;
        })
        .finally(() =>
        {
        });

        return response;
    },


    async setAnnouncementRead (vue_instance , announcementID) { 

        let url = new URL(process.env.VUE_APP_API_URL + "v1/message/announcement/read");
        let dateTime = Date.now();

        let apiKey = [REDACTED];
        let signKey = MD5(`${apiKey}_${dateTime}`).toString();

        let params = {
            signKey: signKey,
            ts: dateTime,
            announcementID: announcementID
        };

        let response = fetch(url, {
            method: "POST",
            headers:  vue_instance.instanceRequestHeaders(),
            body: JSON.stringify(params)
        }).then((res)=>
        {
            return res.json();
        })
        .catch((error)=>
        {
            console.log(error);
            return error;
        })
        .finally(() =>
        {
        });

        return response;

    },

    async setMessageRead (vue_instance , messageID) { 

        let url = new URL(process.env.VUE_APP_API_URL + "v1/message/player-messages/read");
        let dateTime = Date.now();

        let apiKey = [REDACTED];
        let signKey = MD5(`${apiKey}_${dateTime}`).toString();

        let params = {
            signKey: signKey,
            ts: dateTime,
            messageID: messageID
        };

        let response = fetch(url, {
            method: "POST",
            headers:  vue_instance.instanceRequestHeaders(),
            body: JSON.stringify(params)
        }).then((res)=>
        {
            return res.json();
        })
        .catch((error)=>
        {
            console.log(error);
            return error;
        })
        .finally(() =>
        {
        });

        return response;

    }

  }  
  
  export default announcementApi



