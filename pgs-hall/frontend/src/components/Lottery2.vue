<template>
  <div style="padding: 15px; max-width: 650px; margin: 0 auto">
    <div
      class="lottery"
      :style="{ backgroundColor: mergedData.bgColor }"
      style="max-width: 450px; margin: 0 auto"
    >
      <div
        class="turntable"
        :style="{
          background: `url('${mergedData.dotImage}') no-repeat center center / 110%`,
        }"
      ></div>

      <div class="firework" v-show="isShowFirework">
        <img src="https://example.invalid/public/firework_0.gif" />
      </div>

      <canvas
        id="canvas"
        :width="mergedData.width"
        :height="mergedData.height"
        :style="{ transform: `rotate(${rotateDeg}deg)` }"
      ></canvas>
      <img
        v-if="searchEventInfo && searchEventInfo != null"
        :src="mergedData.goImage"
        class="lottery-go"
        @click.stop="startRotation"
        id="lotteryGoPic"
      />
    </div>

    <div class="draw-times" v-if="searchEventInfo && searchEventInfo != null">
      {{ $t("Label_RemainingDrawTimes") }}：<span class="my-remaining-times">
        {{ searchEventInfo.remainingDrawTimes }}
      </span>
      <div style="font-size: 12px; color: #adadad">
        {{ $t("Label_CosumePointPerDraw") }}：{{
          toAmountDisplay(searchEventInfo.pointPerDraw)
        }}
      </div>
      <div style="font-size: 12px; color: #adadad">
        {{ $t("Label_RemainingPoint") }}：
        {{ toAmountDisplay(searchEventInfo.remainingPoint) }}
      </div>
    </div>

    <div
      style="margin-top: 100px"
      v-if="searchEventInfo && searchEventInfo != null"
    >
      <div style="margin: 0 auto; margin-top: 25px">
        <span class="border-line"></span>
        <span class="rules-tile">{{ $t("Label_EventRestrictGames") }}</span>
        <span class="border-line"></span>
      </div>
    </div>
    <div>
      <div class="row">
        <div
          class="col-md-4 col-4"
          v-for="(item, index) in supportGames"
          :key="index"
        >
          <img
            :src="item.imageUrl"
            class="game-img"
            :title="item.gameName"
            @click="clickGameItem(item)"
          />
        </div>

        <div class="col-md-4 col-4" v-if="supportGamesCountModValue() == 1">
          &nbsp;
        </div>

        <div
          class="col-md-4 col-4"
          v-if="
            supportGamesCountModValue() == 2 || supportGamesCountModValue() == 1
          "
        >
          &nbsp;
        </div>
      </div>
    </div>

    <div v-if="isShowPrizeResultModal" id="myPrizeResult" class="modal-overlay">
      <div class="popup-modal">
        <div class="modal-title" style="color: #fff; font-weight: 100">
          <b> {{ $t("Label_DrawResult") }} </b>
        </div>

        <div class="content" v-if="prizeResult && prizeResult != null">
          <div class="resultPrizeRow">
            <div class="resultLabelText">{{ $t("Label_OrderNo") }}</div>
            {{ prizeResult.referenceTransactionNo }}
          </div>

          <div class="resultPrizeRow">
            <div class="resultLabelText">{{ $t("Label_Time") }}</div>
            {{ dateToYYYYMMDDHHMMss(prizeResult.referenceSendTime) }} (GMT+{{
              getClientTimezone()
            }})
          </div>

          <div class="resultPrizeRow">
            <div class="resultLabelText">{{ $t("Label_PrizeName") }}</div>
            {{ prizeMessage }}
          </div>
        </div>

        <div class="text-c" style="margin-top: 0px">
          <button
            @click="closeAllModal()"
            class="modal-button"
            style="background: #ff640a; font-size: 16px"
          >
            {{ $t("Button_Confirm") }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import eventApi from "../utilities/api/eventApi";
import commonMixin from "../utilities/mixin/commonMixin";

export default {
  name: "v-lottery",
  mixins: [commonMixin],
  props: {
    data: {
      type: Object,
      required: true,
      default: () => {},
    },
    disabled: Boolean,
  },
  data() {
    return {
      isShowFirework: false,
      isShowPrizeResultModal: false,
      isLockClick: false,

      prizeResult: null,
      prizeMessage: "",

      isplayed: true,
      flag: false, // 转盘开关
      sound_firework: null,
      sound_rotate: null,
      searchEventInfo: null,
      supportGames: [],
      itemColors: [
        //紅黃綠
        { color1: "#f3393a", color2: "#A01517" },
        { color1: "#FDCF51", color2: "#ED8949" },
        { color1: "#99b161", color2: "#99b161" },
      ],

      eventTypeOfThisPage: 93,
      piece: 0,
      rotateDeg: 0,
      defaultData: {
        data: [],
        target: "",
        bgColor: "#ff5859",
        dotImage: "https://example.invalid/public/wheel1.gif",
        goImage: "https://example.invalid/public/go2.gif",
        width: 500,
        height: 500,
      },
    };
  },
  computed: {
    startAngle() {
      // 獎項漸層轉動
      const areaArc = 360 / this.mergedData.data.length / 2;
      return (areaArc * Math.PI) / 180;
    },
    // 大圆盘半径
    outsideRadius() {
      return (this.mergedData.width * 12) / 30;
    },
    // 字体离圆心距离
    textRadius() {
      return this.mergedData.width * 0.3;
    },
    // 防止默认值被覆盖
    mergedData() {
      return {
        ...this.defaultData,
        ...this.data,
      };
    },
  },
  watch: {
    disabled(val) {
      this.flag = val;
    },
  },
  mounted() {
    var audio1 = new Audio(require("../assets/media/fire.mp3"));
    var audio2 = new Audio(require("../assets/media/yisell.mp3"));

    this.sound_firework = audio1;
    this.sound_rotate = audio2;
    this.initialWheelInfoByApi();
  },

  methods: {
    toAmountDisplay(num) {
      var result = [],
        counter = 0;
      num = (num || 0).toString().split("");
      for (var i = num.length - 1; i >= 0; i--) {
        counter++;
        result.unshift(num[i]);
        if (!(counter % 3) && i != 0) {
          result.unshift(",");
        }
      }
      return result.join("");
    },

    supportGamesCountModValue() {
      if (this.supportGames.length <= 0) {
        return 0;
      }
      return this.supportGames.length % 3;
    },

    toDisplayTimeText(timeText) {
      return timeText.replaceAll("T", " ").replaceAll("Z", "") + " (GMT+0)";
    },

    initialWheelInfoByApi(isInitialCanvas = true) {
      eventApi
        .getSingleEventByType(this, this.eventTypeOfThisPage)
        .then((res) => {
          this.mergedData.data = res.data.luckyWheel.prizeItems;
          this.searchEventInfo = res.data.luckyWheel;
          this.supportGames = res.data.supportGames;

          if (isInitialCanvas) {
            this.initialLotteryCanvas();
          }
        });
    },

    initialLotteryCanvas() {
      let loopIndex = 0;
      this.mergedData.data.forEach((item) => {
        const modValue = loopIndex % 3;
        item.color1 = this.itemColors[modValue].color1;
        item.color2 = this.itemColors[modValue].color2;
        loopIndex++;
      });

      this.setLotteryHeight();
      window.addEventListener("resize", () => {
        this.setLotteryHeight();
      });
      this.drawLottery();
      const canvas = document.getElementById("canvas");
      canvas.addEventListener("transitionend", () => {
        this.flag = false;
        this.whenRotationEnd();
      });
    },

    // 设置转盘的高，使其与宽相等
    setLotteryHeight() {
      const lottery = document.querySelector(".lottery");
      lottery.style.height = lottery.clientWidth + "px";
    },
    // 绘制转盘
    drawLottery() {
      const canvas = document.getElementById("canvas");
      const ctx = canvas.getContext("2d");
      let devicePixelRatio = window.devicePixelRatio || 1;
      let backingStoreRatio =
        ctx.webkitBackingStorePixelRatio ||
        ctx.mozBackingStorePixelRatio ||
        ctx.msBackingStorePixelRatio ||
        ctx.oBackingStorePixelRatio ||
        ctx.backingStorePixelRatio ||
        1;
      let ratio = devicePixelRatio / backingStoreRatio;
      canvas.width = canvas.width * ratio;
      canvas.height = canvas.height * ratio;
      ctx.scale(ratio, ratio);
      ctx.translate(0.5, 0.5);
      this.ctx = ctx;

      const [w, h, centerX, centerY] = [
        this.mergedData.width,
        this.mergedData.height,
        this.mergedData.width / 2,
        this.mergedData.height / 2,
      ];
      this.centerX = centerX;
      this.centerY = centerY;
      const line_height = 17;

      console.log("--------------");
      console.log(this.mergedData.data);

      // 根据奖品个数计算圆周角度
      let arc = Math.PI / (this.mergedData.data.length / 2);
      this.piece = arc;
      ctx.clearRect(0, 0, w, h);
      ctx.strokeStyle = "transparent";
      ctx.font = "23px Microsoft YaHei";

      this.mergedData.data.forEach((item, i) => {
        let angle = this.startAngle + i * arc;
        var bgc = ctx.createRadialGradient(200, 200, 50, 300, 300, 350);
        // 先取得漸層的起點和終點

        bgc.addColorStop(0, item.color1);
        bgc.addColorStop(1, item.color2);

        // 設定漸層的位置和顏色
        ctx.fillStyle = bgc;
        // 把位置和顏色填入
        ctx.fillRect(0, 0, 0, 0);
        // 裁切要漸層的範圍
        // ctx.fillStyle = item.color || "transparent";
        ctx.beginPath();
        ctx.moveTo(centerX, centerY);

        ctx.arc(
          centerX,
          centerY,
          this.outsideRadius,
          angle,
          angle + arc,
          false
        );
        // ctx.arc(centerX, centerY, this.insideRadius, angle + arc, angle, true)
        ctx.closePath();
        ctx.fill();
        ctx.stroke();
        ctx.save();

        this.drawTitle(item, angle, arc);
      });
    },
    // 重置绘图坐标
    resetCoordinate(angle, arc) {
      const ctx = this.ctx;
      let x = this.centerX + Math.cos(angle + arc / 2) * this.textRadius;
      let y = this.centerY + Math.sin(angle + arc / 2) * this.textRadius;
      // translate重新映射画布上的（0，0）位置
      ctx.translate(x, y);
      ctx.rotate(angle + arc / 2 + Math.PI / 2);
    },
    /**
     * 绘制奖品名称
     * @param {Object} item 奖品信息
     * @param {Number} angle 开始绘制的位置角度
     * @param {Number} arc 绘制的弧度
     */
    drawTitle(item, angle, arc) {
      const ctx = this.ctx;
      ctx.fillStyle = "#fff";
      ctx.shadowColor = "#000";
      ctx.shadowBlur = 20;
      ctx.shadowOffsetY = 5;
      ctx.font = "12px";
      this.resetCoordinate(angle, arc);
      ctx.fillText(
        item.displayName,
        -ctx.measureText(item.displayName).width / 2,
        0
      );
      ctx.restore();
      ctx.save();
    },

    // 获取目标角度
    getTargetAngel() {
      let idx = this.mergedData.data.findIndex((item) => {
        return item.id === this.mergedData.target;
      });
      if (idx !== -1) {
        return ((this.piece * (idx + 0.5) + this.startAngle) * 180) / Math.PI;
      } else {
        return 360;
      }
    },

    // 开始旋转
    startRotation() {
      if (this.isLockClick) {
        return;
      }

      this.isLockClick = true;

      eventApi
        .startDraw_LuckyWheel(this)
        .then((res) => {
          if (res.code != "0") {
            alert(res.message);
            this.isLockClick = false;
          } else {
            this.mergedData.target = res.data.itemDetail.eventDrawPrizeItemID;
            this.prizeResult = res.data;
            this.sound_rotate.play();

            if (this.mergedData.target) {
              let finalDeg = this.getTargetAngel();
              this.rotateDeg +=
                270 - finalDeg + (360 - (this.rotateDeg % 360)) + 1800;
              this.setEffects();
            }
          }
        })
        .catch((err) => {
          //alert(err);
        })
        .finally(() => {});
    },

    whenRotationEnd() {
      this.isShowFirework = true;
      this.soundPause(this.sound_rotate);
      this.sound_firework.play();

      //刷新次數
      this.initialWheelInfoByApi(false);

      const myDrawItem = this.mergedData.data.filter(
        (x) => x.id == this.mergedData.target
      );

      //顯示抽到蝦米
      if (myDrawItem && myDrawItem.length > 0) {
        this.prizeMessage = myDrawItem[0].displayName;
        this.isShowPrizeResultModal = true;
      }
      setTimeout(() => {
        this.isLockClick = false;
      }, 1000);

      setTimeout(() => {
        this.isShowFirework = false;
        this.soundPause(this.sound_firework);
      }, 2750);
    },

    soundPause(targetObject) {
      targetObject.currentTime = 0;
      targetObject.pause();
    },

    closeAllModal() {
      this.isShowPrizeResultModal = false;
    },

    async clickGameItem(gameItem) {
      this.redirectToGamePage(gameItem);
    },

    setEffects() {
      if (!this.flag) {
        this.flag = true;
      }
    },
  },
};
</script>
<!-- Add "scoped" attribute to limit CSS to this component only -->
<style scoped>
.lottery {
  position: relative;
  width: 100%;
  height: 100%;
  border-radius: 50%;
}
.fire {
  position: absolute;
  top: -3px;
  right: 0;
  left: 0px;
  width: 100%;
  margin: auto;
  display: none;
}
#canvas {
  width: 92%;
  /* height: 92%; */
  position: absolute;
  top: 4%;
  left: 4%;
  transition: transform 5s ease-in-out;
  /* cubic-bezier(0.48, 0.76, 0.49, 0.97) */
  /* let the browser know we plan to animate
     each view in and out */
  will-change: transform;
}
.turntable {
  height: 100%;
  /* animation: rotate180 7s linear both reverse infinite; */
  /* let the browser know we plan to animate
     each view in and out */
  will-change: transform;
}
.lottery-go {
  position: absolute;
  width: 25%;
  left: 50%;
  top: 47%;
  z-index: 1;
  transform: translate(-50%, -50%);
}
@keyframes rotate180 {
  from {
    transform: rotate(180deg);
  }
  to {
    transform: rotate(0deg);
  }
}

.firework {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  width: 100%;
  height: 100vh;
  z-index: 9999999;
}

.draw-times {
  font-size: 18px;
  margin-top: 15px;
}

.game-img {
  height: 100px;
  border-radius: 5px;
  margin-top: 15px;
  cursor: pointer;
}

@media only screen and (min-width: 480px) {
  .firework img {
    width: 600px;
  }
  .game-img {
    height: 175px;
    border-radius: 5px;
    margin-top: 15px;
  }
}

.rules-tile {
  font-size: 20px;
  font-weight: 100;
  color: #fffa55;
  margin-left: 5px;
  margin-right: 5px;
}

.border-line {
  border-bottom: solid 1px #7c7a39;
  width: 50px;
  display: inline-block;
  margin-bottom: 8px;
}

.resultPrizeRow {
  text-align: left;
  font-size: 14px;
  color: #fff;
  margin-top: 10px;
}

.resultLabelText {
  text-align: left;
  font-size: 11px;
  margin-bottom: -5px;
  color: #aaa;
}

.my-remaining-times {
  font-size: 30px;
  color: #fffa55;
}

#myPrizeResult .popup-modal {
  top: 35vh;
  background-image: linear-gradient(to top, #5e260c, #ff4700);
}

#myPrizeResult .content {
  padding: 5px 10px 25px 10px;
  background: #401717;
  border-radius: 5px;
  border: solid 1px #4c341a;
  margin-top: 5px;
  margin-bottom: 20px;
}
</style>
