<template>
  <div>
    <div
      v-if="isShowUrl"
      style="padding: 10px; font-size: 14px; margin-top: 10%"
    >
      Will redirect to following url in few seconds...
      <div>
        <a :href="myRedirectUrl">{{ myRedirectUrl }}</a>
      </div>
    </div>
  </div>
</template>

<script>
import commonMixin from "../utilities/mixin/commonMixin";

export default {
  components: {},

  mixins: [commonMixin],
  computed: {
    isCN() {
      const lang = localStorage.getItem("LOCALE");

      if (lang === "zh-cn" || lang === "zh-tw") {
        return true;
      }

      return false;
    },
  },

  data() {
    return {
      isLoading: false,
      isShowUrl: false,
      myRedirectUrl: "",
      isMaintainMsgRead: false,
    };
  },

  created() {
    this.redirectPageForIframe();
  },

  methods: {
    //如果判斷當前是iframe 則改變網址的是parent那一層
    redirectPageForIframe() {
      let targetUrl = this.$route.query.backurlBase64;
      let redirectUrl = "";

      if (!targetUrl || targetUrl == null) {
        return;
      } else {
        redirectUrl = atob(targetUrl);
      }
      if (window.self !== example.invalid) {
        window.parent.location.href = redirectUrl;
      } else {
        window.location.href = redirectUrl;
      }

      setTimeout(() => {
        this.isShowUrl = true;
        this.myRedirectUrl = redirectUrl;
      }, 3000);
    },

    closeAllModal() {
      this.isMaintainMsgRead = true;
    },
  },
};
</script>

<style scoped>
@media only screen and (min-width: 0px) and (max-width: 767px) {
  .event-title {
    font-size: 20px !important;
  }
}
</style>
