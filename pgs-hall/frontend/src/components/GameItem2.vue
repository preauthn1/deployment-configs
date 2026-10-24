<template>
  <div class="container v2_marshall">
    <div class="row row-cols-2 row-cols-md-4 g-3">
      <div class="col" v-for="(game, index) in games" :key="index">
        <div class="item">
          <div class="icon-bg p-relative">
            <div
              class="blurBG"
              :style="
                'background-image: url(' +
                replaceToOriginalImageUrl(game.imageUrl) +
                ')'
              "
            ></div>
            <img
              @click="showModal(game)"
              class="icon p-relative"
              :src="game.imageUrl"
            />
          </div>
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
          <div
            class="p-3 text-start descr-panel"
            :data-bs-toggle="isDescriptionNotEmpty(game) ? 'modal' : ''"
            :data-bs-target="
              isDescriptionNotEmpty(game) ? '#gameInfoModal' : ''
            "
            @click="gameInfo = getGameInfo(game)"
          >
            <div class="game-name">
              {{ getGameName(game) }}
            </div>

            <div class="info-row" v-if="isDescriptionNotEmpty(game)">
              <i class="fas fa-info-circle"></i>
              {{ $t("Label_Detail") }}
            </div>

            <div class="game-description" v-if="showDescription">
              {{ getDescription(game) }} &nbsp;
            </div>

            <div
              class="extraInfo-cardDisplay"
              v-if="showDescription && game.isShowExtraInfo"
            >
              <div style="flex: 1">
                <div class="extra-info-value">
                  {{ game.maximumWin }}
                  <span style="font-size: 14px"></span>
                </div>
                <div class="extra-info-label">Maximum Win</div>
              </div>
              <div style="flex: 1">
                <div class="extra-info-value">
                  {{ game.lineWays }}
                </div>
                <div class="extra-info-label">Lines/Ways</div>
              </div>
            </div>
          </div>

          <div class="btn-group">
            <!-- <button
              v-if="showDescription && getDescription(game)"
              class="info-btn"
              data-bs-toggle="modal"
              data-bs-target="#gameInfoModal"
              @click="gameInfo = getGameInfo(game)"
            >
              Info
            </button> -->

            <button
              class="play-btn"
              style=""
              :disabled="game.isMaintain"
              @click="clickGameItem(game)"
            >
              {{ $t("Label_TrialPlay") }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <div
      class="modal fade"
      ref="refGameInfoModal"
      id="gameInfoModal"
      aria-labelledby="gameInfoModalLabel"
      aria-hidden="true"
    >
      <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title" id="gameInfoModalLabel">
              {{ gameInfo?.name }}
            </h5>
          </div>
          <div class="modal-body">
            {{ gameInfo?.desc }}

            <div
              style="margin-top: 30px"
              v-if="gameInfo && gameInfo.gameDetail.isShowExtraInfo"
            >
              <hr />
              <div style="display: flex">
                <div style="flex: 1">
                  <div class="extra-info-value">
                    {{ gameInfo.gameDetail.maximumWin }}
                    <span style="font-size: 14px"></span>
                  </div>
                  <div class="extra-info-label">Maximum Win</div>
                </div>
                <div style="flex: 1">
                  <div class="extra-info-value">
                    {{ gameInfo.gameDetail.lineWays }}
                  </div>
                  <div class="extra-info-label">Lines/Ways</div>
                </div>
              </div>
            </div>
          </div>

          <div class="modal-footer">
            <div class="actionButton close-modal-btn" data-bs-dismiss="modal">
              {{ $t("Label_Cancel") }}
            </div>
            <div
              class="actionButton"
              style="background: #242644"
              @click="clickGameItem(gameInfo.gameDetail)"
            >
              {{ $t("Label_TrialPlay") }}
            </div>
          </div>
          <img
            class="modal-img"
            :src="replaceToOriginalImageUrl(gameInfo?.imageUrl)"
          />
        </div>
      </div>
    </div>

    <Loader v-if="isLoading" />
  </div>
</template>

<script>
import commonMixin from "../utilities/mixin/commonMixin";
import gameDesc from "../static/game_descs.json";
import { bootstrap, Modal } from "bootstrap";

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
      user: "",
      locale: localStorage.getItem("LOCALE"),
      showDescription: window.isShowDescription,
      gameInfo: null,
      isLoading: false,
    };
  },

  computed: {},
  methods: {
    async clickGameItem(gameItem) {
      /*
      this.redirectToGamePage(gameItem);
      */
      this.open(gameItem.gameID, gameItem.servers[0].serverID);
    },

    replaceToOriginalImageUrl(img) {
      try {
        return img.replaceAll("target-game-icon", "game-icon");
      } catch (e) {
        console.log(e);
      }

      return "";
    },

    showModal(game) {
      // 有文案才彈出
      if (this.isDescriptionNotEmpty(game)) {
        this.gameInfo = this.getGameInfo(game);
        const myModel = new Modal(this.$refs.refGameInfoModal);
        myModel.show();
      }
    },

    open(gid, sid) {
      const lang = localStorage.getItem("LOCALE");

      let finalUrl = "";

      if (gid == "310001") {
        finalUrl = "https://example.invalid/index.html";
      } else {
        let domain = window.location.protocol + "//" + window.location.host;
        finalUrl =
          domain +
          `/open?gid=${gid}` +
          `&sid=${sid}` +
          `&user=${this.user}` +
          `&lang=${lang}` +
          `&ts=${Date.now()}`;
      }

      this.isLoading = true;

      setTimeout(() => {
        this.isLoading = false;
      }, 3000);

      //2024/05 調整成不另開視窗
      location.href = finalUrl;

      //window.open(finalUrl);
    },

    getGameInfo(game) {
      const info = gameDesc[game.gameID];

      // debug用
      // debug用
      // debug用
      if (game.gameID == 290105) {
        console.log("--------------");
        console.log(info);
      }

      if (info) {
        return {
          ...info[this.locale],
          imageUrl: game.imageUrl,
          gameDetail: game,
        };
      }
      return null;
    },

    isDescriptionNotEmpty(game) {
      return this.getDescription(game).length > 0;
    },

    getDescription(game) {
      return this.getGameInfo(game)?.desc || "";
    },

    getGameName(game) {
      return this.getGameInfo(game)?.name || game.gameName;
    },
  },
  mounted() {
    const fixedUser = window.fixedUser;
    if (fixedUser && fixedUser != "") {
      this.user = fixedUser;
    } else {
      this.user = "demo-" + Date.now();
    }
  },
};
</script>
<style scoped>
.item {
  background: linear-gradient(180deg, #0c0c0c 0%, #1a1a1a 100%);
  /* border: 1px solid #d1b465; */
  height: 100%;
  border-radius: 8px;
  position: relative;
  padding-bottom: 44px;
}

.icon-bg {
  overflow: hidden;
  position: relative;

  border-top-left-radius: 8px;
  border-top-right-radius: 8px;
  background: linear-gradient(
    180deg,
    #141414 0%,
    #1d1d1d 31%,
    #292929 65%,
    #2f2f2f 93%,
    #292929 100%
  );
}

.icon {
  width: 60%;
  height: 60%;
  object-fit: cover;
  border-radius: 4px;
}

.game-name {
  color: #fff655;
  font-size: 20px;
  font-weight: bold;
}

.game-description {
  min-height: 50px;
  max-height: 75px;
  font-size: 14px;
  cursor: pointer;
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  /* -webkit-mask-image: linear-gradient(180deg, #000 50%, transparent); */
}

.btn-group {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 34px;
}

.btn-group button:first-child {
  border-bottom-left-radius: 6px;
}
.btn-group button:last-child {
  border-bottom-right-radius: 6px;
}

.info-btn,
.play-btn {
  width: 100%;
  border: none;
  font-size: 18px;
}

.info-btn {
  background: linear-gradient(180deg, #7c45c7 0%, #2a0459 71%, #7c45c7 100%);
  color: #fff;
}

.play-btn {
  /* 
  background: linear-gradient(180deg, #f7f1c2 0%, #c4a44b 77%, #dbbd69 100%);
  color: #000; */

  color: #fff;
  border-top-right-radius: 0px;
  border-bottom-right-radius: 0px;
}

.play-btn:disabled {
  background: #929292;
  color: #585858;
}
.modal-title {
  color: #fff;
  font-weight: bold;
}

.modal-content {
  background: #333333;
  color: #fff;
  border-radius: 20px;
}
.modal-img {
  position: absolute;
  top: 0;
  left: 50%;
  transform: translate(-50%, -50%);
  height: 160px;
  width: 160px;
  border-radius: 50%;
}

.modal-header {
  padding-top: 100px;
  border-bottom: none;
  justify-content: center;
}
.modal-body {
  font-size: 16px;
  padding-bottom: 40px;
  padding-left: 2.5rem;
  padding-right: 2.5rem;
}
.modal-footer {
  border-top: 1px solid #fff;
  justify-content: center;
  padding: 20px 0;
}
.close-modal-btn {
  border: 0;
  background: transparent;
  color: #ffffffcc;
  font-size: 16px;
}

.close-modal-btn:hover {
  color: #fff;
}

.icon-bg img {
  min-height: 105px;
}

@media only screen and (min-width: 800px) {
  .icon-bg img {
    min-height: 160px;
  }
}
</style>
