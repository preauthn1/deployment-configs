<template>
  <div
    v-for="(game, index) in games"
    :key="index"
    class="item"
    :class="getAutomatedTestID(game)"
    @click="clickGameItem(game)"
  >
    <div v-if="!game.isMaintain">
      <div class="img-item">
        <!--       
      <div v-if="game.isRecentlyGame" class="history-clock">
        <i class="fas fa-history"></i>
      </div> -->

        <div class="frame">
          <img
            class="tag-hot"
            src="@/assets/images/lobby/tag_hot.png"
            v-if="game.isHotGame && isShowHighLightTag == true"
          />
          <img
            class="tag-new"
            src="@/assets/images/lobby/tag_new.png"
            v-if="game.isNewGame && isShowHighLightTag == true"
          />

          <!-- 
          <img
            class="icon"
            :src="game.gameImageUrl"
            :loading="isLazyLoadSequence(game) ? 'lazy' : 'eager'"
          />
          -->
          <img
            class="icon"
            :src="game.imageUrl"
            :loading="isLazyLoadSequence(game) ? 'lazy' : 'eager'"
          />
        </div>

        <!--  舊版css
      <img class="icon" :src="game.gameImageUrl" />
      <img class="frame" src="../assets/images/lobby/game_item_frame.png" /> -->
      </div>

      <div class="gameName">
        {{ game.gameName }}
      </div>
    </div>

    <div
      class="item"
      :class="getAutomatedTestID(game)"
      v-if="game.isMaintain"
      style="opacity: 0.2; cursor: initial"
    >
      <img class="icon" :src="game.gameImageUrl" />
    </div>
  </div>

  <!-- 讓排版總是保持至少3個items (避免grid css排版亂掉) -->
  <div v-if="games && games.length <= 2" class="item"></div>
  <div v-if="games && games.length == 1" class="item"></div>
</template>

<script>
import commonMixin from "../utilities/mixin/commonMixin";
export default {
  props: [
    //"game",
    "games",
    "isShowHighLightTag",
    "isEnableLazyLoad",
  ],
  mixins: [commonMixin],
  data() {
    return {
      user: '',
    };
  },

  computed: {},
  methods: {
    //後面的圖片延遲載入..但感覺不出效果
    isLazyLoadSequence(game) {
      return this.isEnableLazyLoad && game.sequence > 9;
    },

    async clickGameItem(gameItem) {
      /*
      this.redirectToGamePage(gameItem);
      */
      this.open(gameItem.gameID, gameItem.servers[0].serverID);
    },

    open (gid, sid) {
      const lang = localStorage.getItem("LOCALE");
      let url = window.location.protocol + "//" + window.location.host + "/open?gid=" + gid + "&sid=" + sid  + "&user=" + this.user + "&lang=" + lang + "&ts=" + Date.now()
      if (gid == "310001") url = "https://example.invalid/index.html"; // for fishing king
      window.open(url)
    },

    //QA自動測試識別用
    getAutomatedTestID(game) {
      return "game_" + game.gameID;
    },
  },
  mounted() {
    const fixedUser = window.fixedUser;
      if (fixedUser && fixedUser != "") {
          this.user = fixedUser;
      } else {
          this.user = "demo-" + Date.now()
      }
  },
};
</script>
<style scoped>
.item .img-item {
  width: 20rem;
}

.item .img-item {
  width: 17.5rem;
  height: 17.5rem;
  position: relative;
  /* 
  display: flex; */

  cursor: pointer;
}

.icon {
  margin: auto;
  height: 95%;
  width: 95%;
  margin-top: 0.3rem;
  object-fit: cover;
  /* border-radius: 0.8rem; */
  border-top-left-radius: 20px;
  border-bottom-right-radius: 20px;
  background: #333;
  /*border: solid 1px #807a61; /* 網路速度不佳的淡黃邊框 */
}

.frame {
  position: absolute;
  top: 0;
  left: 0;
  height: 100%;
  width: 100%;
}

@media only screen and (min-width: 0px) and (max-width: 767px) {
  .item {
    width: 12.3rem;
  }

  .item .img-item {
    margin-bottom: 5px;
  }

  .grid-container .item .img-item {
    width: 12.3rem;
    height: 12.3rem;
  }

  .icon {
    margin-top: 0.8rem !important;
  }

  .grid-container3X .item {
    width: 11.8rem;
  }

  .grid-container3X .item .img-item {
    width: 11.8rem;
    height: 11.8rem;
  }
}

@media only screen and (min-width: 768px) and (max-width: 1365px) {
  .item {
    width: 16rem;
  }

  .grid-container .item .img-item,
  .grid-container3X .item .img-item {
    width: 16rem;
    height: 16rem;
  }
}

@media only screen and (min-width: 1366px) {
  .item {
    width: 17.5rem;
  }
}
</style>


