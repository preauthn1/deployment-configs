<template>
  <div class="set-lang-bg">
    <div class="underFixHeader pc-max-width">
      <!-- <div class="label-select-language">選擇語系</div> -->

      <div class="select-language-panel" v-if="isRender">
        <div
          :class="'select-language-row' + isSelectStyleClass('en-us', '')"
          @click="changeLanguage('en-us')"
        >
          <div class="select-language">
            <img src="@/assets/images/language/uk.png" />
            <div class="country">English</div>
          </div>
          <i class="fas fa-check" style="font-size: 18px"></i>
        </div>

        <div
          :class="'select-language-row' + isSelectStyleClass('zh-cn', 'zh-tw')"
          @click="changeLanguage('zh-cn')"
        >
          <div class="select-language">
            <img src="@/assets/images/language/cn.png" />

            <div class="country">簡體中文</div>
          </div>
          <i class="fas fa-check" style="font-size: 18px"></i>
        </div>

        <div
          :class="'select-language-row' + isSelectStyleClass('ms-my', '')"
          @click="changeLanguage('ms-my')"
        >
          <div class="select-language">
            <img src="@/assets/images/language/ms-my.png" />

            <div class="country">Melayu</div>
          </div>
          <i class="fas fa-check" style="font-size: 18px"></i>
        </div>

        <div
          :class="'select-language-row' + isSelectStyleClass('vi-vn', '')"
          @click="changeLanguage('vi-vn')"
        >
          <div class="select-language">
            <img src="@/assets/images/language/vi-vn.png" />

            <div class="country">Tiếng Việt</div>
          </div>
          <i class="fas fa-check" style="font-size: 18px"></i>
        </div>

        <div
          :class="'select-language-row' + isSelectStyleClass('th-th', '')"
          @click="changeLanguage('th-th')"
        >
          <div class="select-language">
            <img src="@/assets/images/language/th-th.png" />

            <div class="country">ภาษาไทย</div>
          </div>
          <i class="fas fa-check" style="font-size: 18px"></i>
        </div>

        <!-- 
        <div
          :class="'select-language-row' + isSelectStyleClass('pt-pt', '')"
          @click="changeLanguage('pt-pt')"
        >
          <div class="select-language">
            <img src="@/assets/images/language/pt-pt.png" />

            <div class="country">Português</div>
          </div>
          <i class="fas fa-check" style="font-size: 18px"></i>
        </div> -->
      </div>

      <div>
        <div
          class="link-text"
          @click="backToPreviousPage()"
          style="font-size: 14px; margin-top: 20px"
        >
          {{ $t("Label_BackToPage") }}
        </div>
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
    currentLanguage() {
      return this.getCurrentLanguage();
    },
  },

  data() {
    return {
      isRender: true,
    };
  },

  created() {
    window.scrollTo(0, 0);
  },

  methods: {
    backToPreviousPage() {
      history.back();
    },

    changeLanguage(lang) {
      this.setLanguage(lang);

      //強制刷新render
      this.isRender = false;
      this.isRender = true;

      this.backToPreviousPage();
    },

    isSelectStyleClass(lang1 = "", lang2 = "") {
      var lang = this.getCurrentLanguage();
      return lang == lang1 || lang == lang2 ? " isSelect " : " ";
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
