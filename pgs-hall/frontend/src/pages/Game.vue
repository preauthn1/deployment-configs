<template>
  <div id="game-container">
    <!-- 部分遊戲左上角會顯示返回大廳按鈕 (若有啟用此設置) -->
    <div
      :class="'float-bakcToLobby ' + landscape_required_cssClass"
      v-if="backUrlLobby && backUrlLobby != null && backUrlLobby.length > 0"
      @click="toLobby()"
    >
      <i class="fas fa-home"></i>
      <div>Home</div>
    </div>

    <iframe
      id="target-game-iframe"
      class="game-iframe"
      :src="gameUrl"
      frameborder="0"
    ></iframe>

    <UnreadMessages />

    <!-- 預載圖片避免圖片很慢才顯示出來 -->
    <div v-if="eventEnable" class="preload-hidden-img">
      <img
        src="../assets/images/event/open_red_001.gif"
        class="preload-hidden-img"
      />

      <img
        src="../assets/images/event/red_envelope_2.png"
        class="preload-hidden-img"
      />
    </div>

    <FloatingButton
      v-if="showFloatingButton"
      :redEnvelopeCount="eventInfo.redEnvelope.redEnvelopeCount"
      @button-click="FloatingButtonClick"
    />
    <RedEnvelopeResult
      :reward="redEnvelopeReward"
      v-if="redEnvelopeReward != null"
      @click-close="RedEnvelopeClose"
    />
  </div>
  <Loader v-if="isLoading" />
</template>

<script>
import FloatingButton from "../components/FloatingButton.vue";
import RedEnvelopeResult from "../components/RedEnvelopeReward.vue";
import UnreadMessages from "../components/UnreadMessages.vue";
import commonMixin from "../utilities/mixin/commonMixin";

export default {
  components: {
    FloatingButton,
    RedEnvelopeResult,
    UnreadMessages,
  },
  mixins: [commonMixin],
  data() {
    return {
      isLandscapeRequired: false,
      isLoading: false,
      backUrlLobby: "",
      gameUrl: "",
      eventInfo: null,
      redEnvelopeReward: null,
      clickTimes: 0,
    };
  },
  computed: {
    landscape_required_cssClass() {
      return this.isLandscapeRequired ? "landscape_required" : "";
    },
    //紅包排程 & 紅包顯示的必要條件
    isRedEnvelopeComponentVisible() {
      return this.eventEnable && !this.isUnderSelfIframe();
    },
    //顯示浮動按鈕
    showFloatingButton() {
      return (
        this.isRedEnvelopeComponentVisible &&
        this.eventInfo.redEnvelope.redEnvelopeCount > 0
      );
    },
    eventEnable() {
      return this.eventInfo != null && this.eventInfo.redEnvelope.isEventEnable;
    },
  },
  created() {
    this.LoadGame();
  },

  async mounted() {
    await this.loadEventInfo();

    //活動有啟用 && 非處於巢狀式pgs-iframe才會顯示紅包
    if (this.isRedEnvelopeComponentVisible) {
      this.RegularCheckActivity();
    }
  },
  methods: {
    toLobby() {
      this.clickTimes = this.clickTimes + 1;

      let relativeUrl = window.location.pathname + "?page=lobby";
      this.$router.push(relativeUrl);
    },
    async LoadGame() {
      this.isLoading = true;
      try {
        let gameLinkResult = await this.$store.dispatch(
          "games/fetchIframeGameLink",
          {
            gameID: this.$route.query.gameID,
            checkCode: this.$route.query.checkCode,
          }
        );

        this.gameUrl = gameLinkResult.url;
        this.backUrlLobby = gameLinkResult.backUrlLobby;
        this.isLandscapeRequired = gameLinkResult.isLandscapeRequired;
      } catch (error) {
        alert(error.message);
      }
      this.isLoading = false;
    },

    async loadEventInfo() {
      try {
        let data = await this.$store.dispatch("games/fetchEventInfo");
        this.eventInfo = data;
      } catch (error) {
        alert(error.message);
      }
    },

    //浮動按鈕點擊
    async FloatingButtonClick() {
      //防呆 避免滑鼠點太快
      if (!this.isLoading) {
        this.isLoading = true;

        try {
          let reward = await this.$store.dispatch(
            "games/eventsLuckyDrawRedEnvelope"
          );
          this.redEnvelopeReward = reward;
          await this.loadEventInfo();
        } catch (error) {
          alert(error.message);
        }
      }

      //避免滑鼠點太快 稍微延遲
      setTimeout(() => {
        this.isLoading = false;
      }, 250);
    },

    //關閉紅包獎勵
    RedEnvelopeClose() {
      this.redEnvelopeReward = null;
    },

    //定時確認活動
    RegularCheckActivity() {
      setTimeout(() => {
        this.loadEventInfo();
        this.RegularCheckActivity();
      }, 5000);
    },
  },
};
</script>

<style scoped>
#game-container {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  background-color: black;
}
.game-iframe {
  width: 100%;
  height: 100%;
}

@media only screen and (orientation: portrait) {
  .landscape_required {
    position: absolute;
    top: 15px;
    right: 10px;
    left: initial;
    max-width: 50px;
    -webkit-transform: rotate(90deg);
    -moz-transform: rotate(90deg);
    -o-transform: rotate(90deg);
    -ms-transform: rotate(90deg);
    transform: rotate(90deg);
  }
}
</style>