import {
    getDeviceWidth,
    getDeviceHeight,
    instanceHeaders
} from "./commonMethods";

let commonMixin = {

    methods: {


        getLanguageByQueryString() {
            const lang = this.$route.query.lang;

            if (!lang) {
                return '';
            }

            if (lang == null) {
                return '';
            }

            return lang;
        },


        setLanguageByQueryString() {
            const lang = this.getLanguageByQueryString();
            if (lang == null || lang == '') return;
            this.setLanguage(lang);
        },

        setLanguage(lang) {
            const supportedLangs = ['zh-cn', 'zh-tw', 'en-us', 'ms-my', 'vi-vn', 'th-th', 'pt-pt'];
            if (!supportedLangs.includes(lang)) {
                console.log('未定義的語系，若有新增語系請修改mixin的防呆code. (' + lang + ')');
                lang = 'en-us';
            }

            localStorage.setItem("LOCALE", lang);
            this.$store.commit("user/setLang", lang);
            this.$i18n.locale = lang;
        },

        getCurrentLanguage()
        {
            return localStorage.getItem("LOCALE");
        },

        getToken: [REDACTED] {
            return this.$store.getters["user/token"];
        },

        notEmptyNullText: function(text) {

            if (text === typeof(undefined))
                return false;

            if (text == null)
                return false;

            if (text == '')
                return false;

            if (text.length <= 0)
                return false;

            return true;
        },


        instanceRequestHeaders() {
            //如果出現 fail to fetch的錯誤、很大的原因可能是塞了亂碼的token在header值 (e.g. queryString裡面有token這個參數)
            const token = [REDACTED]"user/token"];
            return instanceHeaders(token);
        },

        getDeviceWidth() {
            return getDeviceWidth();
        },

        numberWithCommas(x) {
            return x.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
        },

        getDeviceHeight() {
            return getDeviceHeight();
        },

        isUnderIframe()
        {
            try {
                return window.self !== example.invalid;
            } catch (e) {
                return true;
            }
        },        

        isUnderSelfIframe() {
            try {
              let iframe = window.parent.document.getElementById("target-game-iframe");
              return this.isUnderIframe() && iframe.id == "target-game-iframe";
            } catch (e) {
              console.log("fail");
            }
            return false;
        },


        getClientTimezone() {
            
            try {
              var myTimezone = (new Date().getTimezoneOffset() / 60) * -1;
              return myTimezone;
            } catch (e) {
              console.log(e);
            }
      
            return 0;
        },

      
        dateToYYYYMMDDHHMMss(date_IOS_8601) {
            var date = new Date(date_IOS_8601);
            try
            {
                return (
                    date.getFullYear() + "-" +
                    ("0" + (date.getMonth() + 1)).slice(-2) + "-" +
                    ("0" + date.getDate()).slice(-2) + " " +
                    ("0" + date.getHours()).slice(-2) + ":" +
                    ("0" + date.getMinutes()).slice(-2) + ":" +
                    ("0" + date.getSeconds()).slice(-2)
                );
            }
            catch (e) 
            {
              console.log(e);
            }
            return '';           
        },      


        //*請注意這method有至少兩個地方用到。若有使用到新欄位 可能需要兩邊的API都做調整
        async redirectToGamePage(gameItem) {

            //判斷順序 OpenNewTab > Iframe > 原頁載入
            //條件1: 商戶需啟用iframe
            //條件2-1: 該遊戲屬於強制導去iframe頁面 (擇一) e.g. PGS遊戲
            //條件2-2: 目前尚未處於pgs自己的iframe頁面 (擇一)                        
            const lobbySetting= this.$store.getters["games/lobbySetting"];
         
            if (lobbySetting.isOpenNewTab) {
              try {
                let gameLinkResult = await this.$store.dispatch(
                  "games/fetchGameLink",
                  {
                    gameID: gameItem.gameID,
                    checkCode: gameItem.checkCode,
                  }
                );
                window.open(gameLinkResult.url);
                return;
              } catch (error) {
                alert(error.message);
              }
            }
      
            if (lobbySetting.isIframeEnable) {
              if (gameItem.mustShowIframeBackButton || !this.isUnderSelfIframe()) {
                console.log("to iframe page...");
      
                let param = `?page=game&gameID=${gameItem.gameID}&checkCode=${gameItem.checkCode}`;
                this.$router.push(window.location.pathname + param);
                return;
              }
            }
      
            this.isLoading = true;
            try {
              let gameLinkResult = await this.$store.dispatch("games/fetchGameLink", {
                gameID: gameItem.gameID,
                checkCode: gameItem.checkCode,
              });
              window.location = gameLinkResult.url;
            } catch (error) {
              alert(error.message);
            }
            this.isLoading = false;
        },

        checkDevice() {

            try {
                const isMobile = window.matchMedia("only screen and (max-width: 760px)").matches;
                console.log('-----checkDevice----');

                console.log('isMobile:');
                console.log(isMobile);

                if (/Android|webOS|iPhone|iPad|iPod|BlackBerry/i.test(navigator.userAgent) || (/Android|webOS|iPhone|iPad|iPod|BlackBerry/i.test(navigator.platform))) {
                    console.log('userAgent match');
                }

            } catch (e) {
                console.log('error');
                console.log(e);
            }
            
        }


    }

}

export default commonMixin