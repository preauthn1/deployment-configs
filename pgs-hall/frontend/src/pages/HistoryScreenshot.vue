<template>
  <div class="history-page-bg" v-if="seachResult && seachResult.basicInfo">
    <div class="">
      <h3>
        <b>
          {{ $t("Title_ViewHistory") }}
        </b>
      </h3>
    </div>

    <div
      class="video-min-height"
      style="margin-bottom: 20px; background: #3c3c3c"
    >
      <div class="vjs-controls-disabled"></div>

      <video
        id="video"
        style="width: 100%; height: 100%"
        class="
          video-js
          vjs-default-skin vjs-show-big-play-button-on-pause
          video-min-height video-bg
        "
        preload="auto"
      >
        <source
          :src="seachResult.basicInfo.videoUrl"
          type="application/x-mpegURL"
        />

        <!-- <source
          src="https://example.invalid/content/MI201109210084_1/m3u8s/f08e80da-bf1d-4e3d-8899-f0f6155f6efa.m3u8"
          type="application/x-mpegURL"
        /> -->
      </video>
    </div>

    <div class="form-info" style="max-width: 600px; margin: 0 auto">
      <div class="row">
        <div class="col-md-6">
          <div class="label-text">{{ $t("Label_GameName") }}:</div>
          <div class="label-value">{{ seachResult.basicInfo.gameName }}</div>
        </div>

        <div class="col-md-6">
          <div class="label-text">{{ $t("Label_OrderNo") }}:</div>
          <div class="label-value">
            {{ seachResult.basicInfo.betRecordNoPGS }}
          </div>
        </div>

        <div class="col-md-6">
          <div class="label-text">{{ $t("Label_PlayerName") }}:</div>
          <div class="label-value">{{ seachResult.basicInfo.playerName }}</div>
        </div>

        <div class="col-md-6">
          <div class="label-text">{{ $t("Label_BetTime") }} (UTC+0):</div>
          <div class="label-value">{{ seachResult.basicInfo.createTime }}</div>
        </div>

        <div class="col-md-6">
          <div class="label-text">{{ $t("Label_BetAmount") }}:</div>
          <div class="label-value">{{ seachResult.basicInfo.betAmount }}</div>
        </div>

        <div class="col-md-6">
          <div class="label-text">{{ $t("Label_PayoutAmount") }}:</div>
          <div class="label-value">
            {{ seachResult.basicInfo.payoutAmount }}
          </div>
        </div>
      </div>

      <div>
        <hr />
        <div class="row" style="margin-top: 15px">
          <div
            v-for="(item, index) in seachResult.settleResultImageList"
            v-bind:key="index"
            style="text-align: left"
            :class="item.isFullWidthDisplay ? 'col-md-12' : 'col-md-6'"
          >
            <div class="f14">
              {{ item.category }}
            </div>

            <div>
              <img
                class="imgItem"
                v-for="(imgItem, myIndex) in item.images"
                :src="imgItem"
                v-bind:key="myIndex"
              />

              <div
                style="
                  margin-left: 5%;
                  font-size: 16px;
                  display: inline-block;
                  vertical-align: bottom;
                "
              >
                {{ item.memoInfo }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div style="margin-top: 25px">
        <hr />
        <div>
          <table class="table">
            <tr>
              <th>Type</th>
              <th>Bet</th>
              <th>Win</th>
              <th>Tax</th>
            </tr>

            <tr v-for="(item, index) in seachResult.betList" v-bind:key="index">
              <td>
                <div v-if="item.is_image_display">
                  <img
                    v-for="(myImg, myIndex) in item.bet_type_images"
                    v-bind:key="myIndex"
                    :src="myImg"
                    style="height: 50px"
                  />
                </div>
                <div v-else>
                  {{ item.bet_type }}
                </div>
              </td>
              <td>{{ item.bet_amount }}</td>
              <td>{{ item.win_amount }}</td>
              <td>{{ item.tax_amount }}</td>
            </tr>
          </table>
        </div>
      </div>
    </div>
  </div>

  <Loader v-if="isLoading" />
</template>

<script>
import commonMixin from "../utilities/mixin/commonMixin";
import otherApi from "../utilities/api/otherApi";
import videojs from "video.js";
import "videojs-contrib-hls";
import "@videojs/http-streaming";

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
      player: null,
      seachResult: null,
      isLoading: false,
      listOfAnnouncement: [],
    };
  },

  methods: {
    initVideo() {
      // 播放器初始化
      this.$nextTick(() => {
        this.player = videojs(
          "video",
          {
            bigPlayButton: true,
            textTrackDisplay: false,
            posterImage: true,
            errorDisplay: false,
            controlBar: true,
            preload: "auto",
            muted: true, //静音模式 、、 解决首次页面加载播放。
          },
          function () {
            var array = document.querySelectorAll(".vjs-controls-disabled");

            array.forEach((element) => {
              console.log(element.classList);
              element.classList.remove("vjs-controls-disabled");
            });
          }
        );
      });
    },

    isNotEmptyText(text) {
      if (text == null) {
        return false;
      }
      if (text === "") {
        return false;
      }
      if (text.length <= 0) {
        return false;
      }

      return true;
    },

    searchData() {
      const transactionNo = this.$route.query.transactionNo;
      const eazyToken = [REDACTED];

      this.isLoading = true;

      otherApi
        .getLiveGameHistoryInfo(this, transactionNo, eazyToken)
        .then((res) => {
          if (res.code != 0) {
            alert(res.message);
          }
          this.seachResult = res.data;
          this.initVideo();
        })
        .catch((res) => {
          console.log(res);
          alert("Fail. System under maintain");
        })
        .finally(() => {
          this.isLoading = false;
        });
    },

    closeAllModal() {},
  },

  created() {
    this.searchData();
  },
};
</script>

<style scoped>
@media only screen and (min-width: 0px) and (max-width: 767px) {
  .event-title {
    font-size: 20px !important;
  }
}
/* 
.vjs-paused.vjs-has-started .vjs-big-play-button {
  display: block;
} */

.video-bg {
  border: solid 2px #222;
}

table {
  margin-top: 25px;
  color: #fff;
  text-align: left;
  font-size: 16px;
}

table th {
  color: #aaa;
}

.vjs-default-skin {
  background: #111;
}

.table > :not(:first-child) {
  border-top: 1px solid currentColor;
  border-color: #333;
}

.imgItem {
  height: 70px;
  margin-right: 3px;
}
</style>

