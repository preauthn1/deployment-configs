<template>
  <div class="banner">
    <transition name="fade" v-for="(value, key) in bannerList" :key="key">
      <img
        :src="value"
        class="w-100"
        v-show="key == gameType"
      />
    </transition>
  </div>
</template>

<script>
export default {
  props: {
    gameType: Number,
  },
  data() {
    return {
      bannerList: {
        "-1" : `${location.origin}/static2/${window.brandName}/banners/-1.jpg`, 
        "1000" : `${location.origin}/static2/${window.brandName}/banners/1000.jpg`, 
        "2000" : `${location.origin}/static2/${window.brandName}/banners/2000.jpg`, 
        "2500" : `${location.origin}/static2/${window.brandName}/banners/2500.jpg`, 
        "3000" : `${location.origin}/static2/${window.brandName}/banners/3000.jpg`, 
        "7000" : `${location.origin}/static2/${window.brandName}/banners/7000.jpg`,
      },
    };
  },
};
</script>

<style scoped>
.banner {
  width: 100%;
  padding-top: 32%;
  position: relative;
}
.banner img {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.fade-enter-to,
.fade-leave-from {
  opacity: 1;
}
</style>