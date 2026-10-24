<template>
  <div class="lobby-bg"></div>
  <div id="main-content" class="main-content">
    <FixHeader />
    <SetLanguage v-if="isLanguagePage" />
    <Lobby v-else />
  </div>
  <!-- 
  <div v-if="allowAnonymous">
    <EventDescription v-if="isEventDescriptionPage" />
    <HistoryScreenshot v-else-if="isScreenshotPage" />
    <RedirectPortal v-else-if="isRedirectPortalPage" />
    <div v-else></div>
  </div>
  <div v-else-if="missingUserinfo" class="missing-message">
    {{ $t("Invalid_AuthToken") }}
  </div>
  <div v-else id="main-content" class="main-content">
    <FixHeader v-if="lobbyStyle == 11 && isShowFixHeaderPage" />
    <Game v-if="isGamePage" />
    <EventLuckyWheel v-else-if="isEventLuckyWheelPage" />
    <Messages v-else-if="isMessagesPage" />
    <SetLanguage v-else-if="isLanguagePage" />
    <Lobby v-else />
  </div>
  -->
</template>

<script>
import commonMixin from "../utilities/mixin/commonMixin";
import FixHeader from "../components/FixHeader.vue";
import EventDescription from "./EventDescription.vue";
import EventLuckyWheel from "./EventLuckyWheel.vue";
import Messages from "./Messages.vue";
import RedirectPortal from "./RedirectPortal.vue";
import SetLanguage from "./SetLanguage.vue";
import Lobby from "./Lobby.vue";
import Game from "./Game.vue";
import HistoryScreenshot from "./HistoryScreenshot.vue";

export default {
  components: {
    Lobby,
    Game,
    EventDescription,
    EventLuckyWheel,
    SetLanguage,
    FixHeader,
    Messages,
    RedirectPortal,
    HistoryScreenshot,
  },
  mixins: [commonMixin],
  computed: {
    isShowFixHeaderPage() {
      return (
        this.isMessagesPage ||
        this.isLanguagePage ||
        this.isLobbyPage ||
        this.isEventLuckyWheelPage
      );
    },

    isLobbyPage() {
      const pageLower = this.getPageParameterLower();
      return pageLower === "lobby" || pageLower === "" || pageLower == null;
    },

    isGamePage() {
      const pageLower = this.getPageParameterLower();
      return pageLower === "game";
    },

    isEventDescriptionPage() {
      const pageLower = this.getPageParameterLower();
      return pageLower === "eventdescription";
    },

    isEventLuckyWheelPage() {
      return this.isTargetPage("EventLuckyWheel");
    },

    isMessagesPage() {
      const pageLower = this.getPageParameterLower();
      return pageLower === "messagelist";
    },

    isLanguagePage() {
      const pageLower = this.getPageParameterLower();
      return pageLower === "language";
    },

    isScreenshotPage() {
      const pageLower = this.getPageParameterLower();
      return pageLower === "historyscreenshot";
    },

    isRedirectPortalPage() {
      const pageLower = this.getPageParameterLower();
      return pageLower === "redirectportal";
    },

    balance() {
      const balance = this.$store.getters["games/balance"];
      return balance;
    },

    playerName() {
      const playerName = this.$store.getters["user/playerName"];
      return playerName;
    },

    lobbyStyle() {
      const lobbySetting = this.$store.getters["games/lobbySetting"];
      return lobbySetting.lobbyStyle;
    },

    token() {
      const token = [REDACTED]"user/token"];
      return token;
    },

    //部分頁面允許不帶token
    allowAnonymous() {
      if (
        this.isEventDescriptionPage ||
        this.isScreenshotPage ||
        this.isRedirectPortalPage
      ) {
        return true;
      }
      return false;
    },

    missingUserinfo() {
      return (
        this.playerName == "" ||
        this.playerName == null ||
        this.token == "" ||
        this.token == null ||
        this.balance == -1
      );
    },
  },

  methods: {
    isTargetPage(myPageName) {
      const pageLower = this.getPageParameterLower();
      return pageLower == myPageName.toLowerCase();
    },

    async getBalanceAndSettings() {
      this.isLoading = true;

      try {
        await this.$store.dispatch("games/loadBalanceAndSettings");
      } catch (error) {
        alert(error.message);
      }

      this.isLoading = false;
    },

    sendNotify() {
      let apiUrl = process.env.VUE_APP_API_URL;

      if (
        apiUrl.includes("localhost") ||
        apiUrl.includes("api.dev.") ||
        apiUrl.includes("0.0.0.0")
      ) {
        alert(
          "(技術提示) querystring有帶token參數。不建議此命名、會與其他邏輯發生衝突。可能導致fail to fetch.(base64 decode)"
        );
      }
    },

    getPageParameterLower() {
      if (!this.$route.query.page) {
        return "";
      }
      const page = this.$route.query.page;

      if (page == null || page === "") {
        return "";
      }

      return page.toLowerCase();
    },
  },

  data() {
    return {
      gameID: this.$route.query.gameID,
    };
  },
  created() {
    /*
    this.setLanguageByQueryString();

    let playerName = this.$route.query.playerName;
    let token = [REDACTED];
    let pgsToken = [REDACTED];
    let lang = this.$route.query.lang;

    //被這錯誤浪費一堆時間 加個防呆提示訊息(開發提示用)
    setTimeout(() => {
      if (this.allowAnonymous && token != null && token != "") {
        this.sendNotify();
      }
    }, 500);

    if (playerName != null) {
      this.$store.commit("user/setPlayerName", playerName);
    }
    // token: [REDACTED]
    // pgsToken: [REDACTED]
    if (token != null) {
      token = [REDACTED]; //base64 decode
      this.$store.commit("user/setToken", token);
    } else if (pgsToken != null) {
      pgsToken = [REDACTED]; //base64 decode
      this.$store.commit("user/setToken", pgsToken);
    }
    setTimeout(() => {
      if (!this.allowAnonymous) {
        this.getBalanceAndSettings();
      }
    }, 100);

    if (lang != null) {
      this.$store.commit("user/setLang", lang);
    }
    */
    document.title = window.title;
    const lang = localStorage.getItem("LOCALE");
    this.setLanguage(lang);
  },
};
</script>

<style>
.missing-message {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100vh;
  font-size: 1.8rem;
  padding: 3% 8%;
}

.lobby-bg:before {
  content: " ";
  position: fixed;
  z-index: -1;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  background: url("../assets/images/lobby/lobby_bg_LQ.jpg") center 0 no-repeat; /* 手機版用低畫質即可 */
  background-size: cover;
}

@media only screen and (min-width: 1366px) {
  .lobby-bg:before {
    background: url("../assets/images/lobby/lobby_bg.jpg") center 0 0 no-repeat;
  }
}

/* 透過css固定rotate的版型 ps.效果不如預期 */
/* @media screen and (min-width: 320px) and (max-width: 850px) and (orientation: landscape) {
  html {
    transform: rotate(-90deg);
    transform-origin: left top;
    width: 100vh;
    overflow-x: hidden;
    position: absolute;
    top: 100%;
    left: 0;
  }
} */
</style>