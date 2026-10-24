<template>
  <img class="language col-2 pointer" :src="getLangImg" @click="clickLang" />
</template>

<script>
export default {
  computed: {
    getLangImg() {
      const lang = this.$store.getters["user/lang"];
      if (lang === "zh-cn" || lang === "zh-tw") {
        return require("@/assets/images/language/cn.png");
      }
      if (lang === "ms-my") {
        return require("@/assets/images/language/ms-my.png");
      }
      if (lang === "vi-vn") {
        return require("@/assets/images/language/vi-vn.png");
      }
      if (lang === "th-th") {
        return require("@/assets/images/language/th-th.png");
      }
      if (lang === "pt-pt") {
        return require("@/assets/images/language/pt-pt.png");
      }
      return require("@/assets/images/language/uk.png");
    },
  },
  methods: {
    clickLang() {
      if (this.$route.query.page == "language") {
        this.$router.push(window.location.pathname);
      } else {
        this.$router.push(window.location.pathname + "?page=language");
      }
    },
  },
};
</script>

<style  scoped>
.language {
  height: 30px;
  width: 30px;
  border-radius: 50%;
}
.pointer {
  cursor: pointer;
}
</style>