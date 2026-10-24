<template>
  <div class="fixHeader">
    <div class="headerLayout">
      <div class="header-left" @click="showMenuBar()">
        <!-- 
        <i class="fas fa-bars menu-bar" style="font-size: 24px"></i>
         -->
      </div>

      <div class="header-middle">
        <a :href="logoLink">
          <img
            :src="logoSrc"
            style="cursor: pointer;"
          />
        </a>
      </div>

      <div class="header-right">
        <!-- <div style="white-space: nowrap"> -->
        <!-- <i class="fas fa-yen-sign"></i> -->
        <!-- {{ numberWithCommas(balance) }}
        </div> -->
        <div class="d-flex flex-row justify-content-end">
        <!-- 
          <PlayerInfo class="d-none d-md-flex col-11" />
         -->
          <HeaderLang class="col-1" />
        </div>
      </div>
    </div>
  </div>

  <!-- 
  <MenuBar ref="refMenuBar" />
   -->
</template>

<script>
import MenuBar from "../components/MenuBar.vue";
import commonMixin from "../utilities/mixin/commonMixin";
import PlayerInfo from "../components/PlayerInfo.vue";
import HeaderLang from "../components/HeaderLang.vue";

export default {
  mixins: [commonMixin],
  components: { MenuBar, PlayerInfo, HeaderLang },
  data() {
    return {
      animatedNum: false,
      logoLink: `${window.brandLink}`.length > 0 ? window.brandLink : "#",
      logoSrc: `${location.origin}/static2/${window.brandName}/logo.png`,
    };
  },

  created() {
    this.setHeight();
  },

  mounted() {},

  methods: {
    setHeight() {
      // First we get the viewport height and we multiple it by 1% to get a value for a vh unit
      let vh = window.innerHeight * 0.01;
      // Then we set the value in the --vh custom property to the root of the document
      document.documentElement.style.setProperty("--vh", `${vh}px`);
    },

    showMenuBar() {
      this.$refs.refMenuBar.showMenuBar();
    },
  },
};
</script>

<style lang="scss" scoped>
</style>