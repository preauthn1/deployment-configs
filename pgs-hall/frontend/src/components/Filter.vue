<template>
  <div class="gametype">
    <div
      v-for="item in gameTypes"
      :key="item.gameTypeName"
      class="gametype-item"
      :class="{ selected: currentGameType == item.gameType }"
      @click="selectGameType(item.gameType)"
    >
      <img v-if="currentGameType == item.gameType" :src="item.selectedImg" />
      <img v-if="currentGameType != item.gameType" :src="item.img" />
      <div>
        {{ item.gameTypeName }}
      </div>
    </div>
  </div>
</template>

<script>
export default {
  computed: {
    gameTypes() {
      const types = this.$store.getters["games/gameTypes"];
      types.unshift({ gameType: -1, gameTypeName: this.$t("GameType_ALL") });
      for (const type of types) {
        try {
          const filename = `${type.gameType}.png`;
          type.img = require(`@/assets/images/gametype/${filename}`);
          type.selectedImg = require(`@/assets/images/gametype/s${filename}`);
        } catch (_) {
          //
        }
      }
      return types;
    },
  },

  data() {
    return {
      currentGameType: -1,
    };
  },
  methods: {
    selectGameType(gameType) {
      this.currentGameType = gameType;
      this.$router.replace('/?gameType=' + gameType);
      this.$emit("select", gameType);
    },
  },
  created() {
    let queryGameType = this.$route.query.gameType;
    if (queryGameType != null) {
      queryGameType = parseInt(queryGameType);
      this.selectGameType(queryGameType);
    }
  },

  mounted() {
    const selectedElement = document.querySelector('.gametype-item.selected');
    if (selectedElement) {
      selectedElement.scrollIntoView({behavior: "smooth", block: "center", inline: "center"});
    }
  },
};
</script>

<style scoped>
.gametype {
  margin-top: 4px;
  margin-bottom: 8px;
  display: flex;
  overflow-x: auto;
  flex-wrap: nowrap;
}

.gametype-item {
  display: inline-block;
  padding: 3px;
  margin: 6px;
  font-size: 14px;
  border-radius: 10px;
  cursor: pointer;
  min-width: 90px;
  height: 85px;
  background: linear-gradient(to bottom, #0c0c0c, #1a1a1a);
  box-shadow: 0px 3px 6px #0000005e;
  text-align: center;
}

.gametype-item img {
  width: 70px;
  height: 55px;
  object-fit: contain;
}

.selected {
  background: linear-gradient(to bottom, #f7f1c2, #c4a44b, #dbbd69);
  color: #000;
  font-weight: bold;
}

@media only screen and (min-width: 768px) {
  .gametype {
    overflow-x: auto;
    flex-wrap: nowrap;
  }

  .gametype {
    text-align: left;
    padding: 8px 0;
  }

  .gametype-item {
    padding: 4px 8px;
    border-radius: 20px;
    min-width: auto;
    height: auto;
    white-space: nowrap;
  }

  .gametype-item img {
    display: none;
  }

  .selected {
    background: #ffbb00;
  }
}
</style>
