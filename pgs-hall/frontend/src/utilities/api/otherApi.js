import { MD5 } from "crypto-js";
import { getGlobalSettings  } from "@/utilities/mixin/commonMethods";

let otherApi = {
   
    async getLiveGameHistoryInfo (vue_instance , transactionNo , token) { 

        let url = new URL(process.env.VUE_APP_API_URL + "v1/bet-records/screenshot/aggLive");
        let dateTime = Date.now();

        let apiKey = [REDACTED];
        let signKey = MD5(`${apiKey}_${dateTime}`).toString();

        let params = {
            signKey: signKey,
            ts: dateTime,
            transactionNo : transactionNo,
            token : [REDACTED]
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
            alert("Fail. System under maintain");
            console.log(error);
            return error;
        })
        .finally(() =>
        {
        });

        return response;
    },


  }  
  
  export default otherApi



