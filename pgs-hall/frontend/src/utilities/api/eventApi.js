// import { getGlobalSettings  } from "@/utilities/mixin/commonMethods";

let eventApi = {

    async getEventDescription (vue_instance , eventID ,  isPreview = false , checkCode = '' , targetLanguage = '') { 

        const parameters = "eventID=" + eventID + '&checkCode=' + checkCode + '&isPreview=' +isPreview.toString() + '&targetLanguage=' + targetLanguage;
        let url = new URL(process.env.VUE_APP_API_URL + "v1/events/description?" + parameters );
       
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



    async getSingleEventByType(vue_instance , eventType) { 

        let url = new URL(process.env.VUE_APP_API_URL + "v1/events/event-info-detail/" +eventType.toString());
       
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


    async getPlayerMerchantEnableEvents(vue_instance) { 

        let url = new URL(process.env.VUE_APP_API_URL + "v1/events/enable-events");
       
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



    async startDraw_LuckyWheel(vue_instance) { 

        let url = new URL(process.env.VUE_APP_API_URL + "v1/events/lucky-draw/LuckyWheel");
                       
        var params = 
        {
            ts : 123
        };

        let response = fetch(url, {
            method: "POST",
            body: JSON.stringify(params),
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




    getToken(vue_instance)
    {
        return vue_instance.$store.getters["user/token"];
    }

  }  
  
  export default eventApi



