<template>
  <div class="underFixHeader">
    <Lottery2
      :data="awardsData"
      @onstart="startRotation"
      @onstop="stopRotation"
    />
  </div>
</template>

<script>
import commonMixin from "../utilities/mixin/commonMixin";
import Lottery2 from "../components/Lottery2.vue";

export default {
  components: { Lottery2 },

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
      test: "",
    };
  },

  mounted() {},

  created() {},

  methods: {},
};
</script>

<style scoped>
</style>
