<template>
  <div class="bg" @click="clickClose">
    <div class="container">
      <div v-show="isStep1" class="none-select">
        <img
          src="../assets/images/event/open_red_001.gif"
          class="redEnv-loading-progress"
        />
      </div>

      <div v-if="isStep2" :class="'result coin-fall-bg'">
        <div>
          <img
            class="close-btn"
            src="../assets/images/event/close.png"
            @click="clickClose"
          />
          <img
            class="redenvelope bounce-effect"
            :src="rewardImage"
            v-on:click.stop
          />

          <div v-show="isShowEarnRewardDiv" style="margin-top: -10px">
            <div>
              <span style="font-size: 16px; margin-right: 5px; color: #ccc">
                {{ $t("Label_EarnReward") }}:
              </span>
              <span class="float-reward-text" id="targetId">0</span>
            </div>
            <div class="sendTimeHint" style="">
              {{ $t("Msg_RewardHadSendToWallet") }}
              (
              {{ dateToYYYYMMDDHHMMss(reward.referenceSendTime) }}
              GMT+{{ getClientTimezone() }}
              )
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import mergeImages from "merge-images";
import { CountUp } from "countup.js";
import commonMixin from "../utilities/mixin/commonMixin";
export default {
  props: ["reward"],
  mixins: [commonMixin],
  data() {
    return {
      isLockClosePopup: false,
      isShowEarnRewardDiv: false,
      isStep1: true,
      isStep2: false,
      rewardImage: require("../assets/images/event/red_envelope_2.png"),
    };
  },
  mounted() {
    this.regenerateImage();
  },

  methods: {
    async regenerateImage() {
      //至少要2秒後才允許關閉(避免玩家沒看到結果就不小心關閉)
      this.isLockClosePopup = true;

      setTimeout(() => {
        this.isLockClosePopup = false;
      }, 2000);

      //顯示拆紅包的動畫(持續1~2秒)
      setTimeout(() => {
        this.isStep1 = false;
        this.isStep2 = true;
        this.setAmountTextFloatAnimation();
        this.playCoinSound();
      }, 1500 * 1);

      setTimeout(() => {
        this.callAGGApiToRereshBalance();
      }, 2000 * 1);

      const gameName = this.textToPicture(this.reward.gameName);
      const orderID = this.textToPicture(
        this.reward.referenceTransactionNo,
        33
      );
      const bet = this.textToPicture(this.reward.referenceBetAmount);
      const bonusRate = this.textToPicture(this.reward.prizeRate);
      const amount = this.textToPicture(this.reward.prizeAmount);
      let localeStr = this.getLocaleString(
        this.reward.referenceTransactionTime
      );
      const date = this.textToPicture(localeStr);
      this.rewardImage = await mergeImages([
        { src: require("../assets/images/event/red_envelope_2.png") },
        { src: gameName, x: 155, y: 570 },
        { src: orderID, x: 155, y: 700 },
        { src: bet, x: 155, y: 820 },
        { src: bonusRate, x: 430, y: 820 },
        { src: amount, x: 155, y: 945 },
        { src: date, x: 155, y: 1060 },
      ]);
    },

    setAmountTextFloatAnimation() {
      try {
        setTimeout(() => {
          document.getElementById("targetId").innerHTML = "0";

          const options = {
            decimalPlaces: 2,
            duration: 2,
          };

          const countUp = new CountUp(
            "targetId",
            this.reward.prizeAmount,
            options
          );

          if (!countUp.error) {
            this.isShowEarnRewardDiv = true;
            countUp.start();
          } else {
            this.isShowEarnRewardDiv = false;
            console.log("error");
            console.error(countUp.error);
          }
        }, 150);
      } catch (e) {
        console.log(e);
      }
    },

    textToPicture(text, fontsize = 40) {
      var canvas = document.createElement("canvas");
      canvas.width = 620;
      canvas.height = 50;
      var ctx = canvas.getContext("2d");
      ctx.font = `bold ${fontsize}px Arial`;
      ctx.fillStyle = "#C22421";
      ctx.fillText(text, 0, 40);
      return canvas.toDataURL();
    },

    //呼叫AGG Api刷新遊戲畫面的餘額
    callAGGApiToRereshBalance() {
      try {
        let gameUrl = document.getElementById("target-game-iframe").src;
        let domain = new URL(gameUrl).origin;

        window.frames[0].postMessage({ method: "updateCoin" }, domain);
      } catch (e) {
        console.log(e);
      }
    },

    getLocaleString(value) {
      let time = new Date(value);
      return time.toLocaleString();
    },

    playCoinSound() {
      try {
        var audio = new Audio(require("../assets/media/coin_sound_001.mp3"));
        audio.play();

        const startVolumeDown = 2000; //幾秒後開始降低音量

        setTimeout(() => {
          audio.volume = 0.6;
        }, startVolumeDown);

        setTimeout(() => {
          audio.volume = 0.4;
        }, startVolumeDown + 500);

        setTimeout(() => {
          audio.volume = 0.2;
        }, startVolumeDown + 1000);

        setTimeout(() => {
          audio.volume = 0.1;
        }, startVolumeDown + 1500);

        setTimeout(() => {
          audio.pause();
        }, startVolumeDown + 2000);
      } catch (e) {
        console.log(e);
      }
    },

    clickClose() {
      if (this.isLockClosePopup) {
        return;
      }

      this.$emit("click-close");
    },
  },
};
</script>

<style scoped>
.coin-fall-bg {
  background-image: url("~@/assets/images/event/red_001.gif");
}

.redEnv-loading-progress {
  width: 150px;
  margin-top: 10%;
}

@media only screen and (max-width: 600px) {
  .redEnv-loading-progress {
    margin-top: 33%;
  }
}

.bg {
  z-index: 999;
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.8);
}
.container {
  position: relative;
  width: 100%;
  height: 100%;
  animation: bounce-in 0.5s;
}

.bounce-effect {
  animation: bounce-in 0.5s;
}

.redenvelope {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}
.result {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 100vw;
  padding-left: 2%;
  padding-right: 2%;
  height: 95vh;
  max-width: 500px;
  max-height: 650px;
  transform: translate(-50%, -50%);
}
.close-btn {
  position: absolute;
  top: 0%;
  right: 2%;
  width: 8%;
  max-width: 25px;
  cursor: pointer;
}

.float-reward-text {
  font-size: 32px;
  color: #fbff00;
  text-shadow: rgb(164 179 127) 0.1em 0.1em 0.2em;
}

.sendTimeHint {
  color: #ccc;
  margin-top: -5px;
}

@keyframes bounce-in {
  0% {
    transform: scale(0);
  }
  50% {
    transform: scale(1.3);
  }
  100% {
    transform: scale(1);
  }
}
</style>