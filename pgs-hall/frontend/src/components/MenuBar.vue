<template>
  <div class="full-overlay" v-if="isShowMenuBar">
    <div class="flex">
      <div class="left">
        <div class="closeButton" @click="closeAllDialog()">
          <i class="fas fa-times-circle" style="font-size: 24px"></i>
        </div>

        <div class="barItems">
          <div class="barItem" @click="toLobby()">
            <i class="fas fa-home"></i>
            <span class="barName"> {{ $t("Label_GameHall") }} </span>
          </div>
          <!-- 
          <div class="barItem">
            <i class="fas fa-award"></i>
            <span class="barName">  {{ $t("Label_Events") }} </span>
          </div>
        -->

          <div
            class="barItem"
            @click="toLuckyWheelPage()"
            v-if="isEnableLuckyWheel"
          >
            <i class="fas fa-gift"></i>
            <span class="barName"> {{ $t("Label_LuckyWheel") }} </span>
          </div>

          <div class="barItem" @click="toMessagePage()">
            <i class="fas fa-envelope"></i>
            <span class="barName"> {{ $t("Label_Messages") }} </span>
          </div>

          <div class="barItem" @click="toLanguagePage()">
            <i class="fas fa-globe"></i>
            <span class="barName"> {{ $t("Label_SelectLanguage") }} </span>
          </div>

          <div
            class="barItem"
            @click="exitToExternal()"
            v-if="isExitButtonVisible"
          >
            <i class="fas fa-sign-out-alt"></i>
            <span class="barName"> {{ $t("Label_Exit") }} </span>
          </div>
        </div>
      </div>
      <div class="right" @click="closeAllDialog()"></div>
    </div>
  </div>
</template>

<script>
import eventApi from "../utilities/api/eventApi";
import commonMixin from "../utilities/mixin/commonMixin";

export default {
  props: ["redEnvelopeCount"],
  mixins: [commonMixin],
  computed: {
    isExitButtonVisible() {
      const lobbySetting = this.$store.getters["games/lobbySetting"];

      if (!lobbySetting) {
        return false;
      }
      const linkUrl = lobbySetting.externalExitLink;

      if (!linkUrl) {
        return false;
      }

      if (linkUrl == "" || linkUrl == null) {
        return false;
      }

      return linkUrl.length > 0;
    },
  },
  data() {
    return {
      isEnableLuckyWheel: false,
      isShowMenuBar: false,
      animatedNum: false,
    };
  },

  mounted() {
    eventApi.getPlayerMerchantEnableEvents(this).then((res) => {
      if (res.code == 0) {
        this.isEnableLuckyWheel = res.data.isEnableLuckyWheel;
      }
    });
  },

  methods: {
    showMenuBar() {
      this.isShowMenuBar = true;
    },

    closeAllDialog() {
      this.isShowMenuBar = false;
    },

    toLobby() {
      this.isShowMenuBar = false;
      this.$router.push(window.location.pathname);
    },

    exitToExternal() {
      let setting = this.$store.getters["games/lobbySetting"];
      let redirectUrl = setting.externalExitLink;

      if (window.self !== example.invalid) {
        window.parent.location.href = redirectUrl;
      } else {
        window.location.href = redirectUrl;
      }
    },

    toLuckyWheelPage() {
      this.isShowMenuBar = false;
      this.$router.push(window.location.pathname + "?page=EventLuckyWheel");
    },

    toMessagePage() {
      this.isShowMenuBar = false;
      this.$router.push(window.location.pathname + "?page=messageList");
    },

    toLanguagePage() {
      this.isShowMenuBar = false;
      this.$router.push(window.location.pathname + "?page=language");
    },
  },
};
</script>

<style lang="scss" scoped>
</style>