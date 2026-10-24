<template>
  <div class="eventDescriptionPage">
    <div
      style="width: 100%"
      v-if="eventDescription && eventDescription != null"
    >
      <!--       
      <div style="text-align: left; padding: 6px 6px 0px 6px; font-size: 12px">
        target X ION
      </div> -->

      <div style="padding: 0px" v-if="eventDescription.topTitle">
        <img
          v-if="isNotEmptyText(eventDescription.topTitle.imageUrlMiddle)"
          class="w100"
          :src="eventDescription.topTitle.imageUrlMiddle"
        />
      </div>

      <div class="text-c" v-if="eventDescription.topTitle">
        <div class="event-title">
          <div style="display: inline-block; position: relative">
            {{ eventDescription.topTitle.title }}
          </div>
        </div>

        <div class="event-time">
          {{ $t("Label_EventTime") }}
        </div>

        <div style="font-size: 12px">
          {{ eventDescription.displayBeginTimeTW }} ~
          {{ eventDescription.displayEndTimeTW }}
        </div>
      </div>

      <div class="section-panel">
        <div v-for="(item, index) in eventDescription.contents" :key="index">
          <!-- 版型51-標準版說明區塊 -->
          <div class="content" v-if="item.layoutType == 51">
            <div>
              <img
                v-if="isNotEmptyText(item.imageUrlTop)"
                class="img-top"
                :src="item.imageUrlTop"
              />
            </div>

            <div class="section-title" v-if="isNotEmptyText(item.title)">
              {{ item.title }}
            </div>

            <div
              class="text-content"
              v-if="isNotEmptyText(item.description)"
              v-html="item.description"
            ></div>

            <div>
              <img
                v-if="isNotEmptyText(item.imageUrlBottom)"
                class="img-bottom"
                :src="item.imageUrlBottom"
              />
            </div>

            <div
              v-if="item.isForPreviewOnly"
              class="text-c"
              style="color: #ff4a4a; font-size: 12px; padding: 5px"
            >
              僅在預覽模式顯示
            </div>
          </div>

          <!-- 版型71-注意事項 -->
          <div class="ruleHint" v-if="item.layoutType == 71">
            <div class="mb-2 text-c">
              <div class="hint-title">
                {{ $t("Label_EventNote") }}
              </div>
            </div>

            <div class="text-content">
              <div v-html="item.description"></div>
            </div>

            <div
              v-if="item.isForPreviewOnly"
              class="text-c"
              style="color: #ff4a4a; font-size: 12px; padding: 5px"
            >
              僅在預覽模式顯示
            </div>
          </div>

          <div class="v-divide"></div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import commonMixin from "../utilities/mixin/commonMixin";
import eventApi from "../utilities/api/eventApi";

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
      eventDescription: null,
      isLoading: false,
      isMaintainMsgRead: false,
      listOfAnnouncement: [],
    };
  },

  created() {
    this.getEventDescription();
  },

  methods: {
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

    getIsPreviewParameter() {
      const isPreview = this.$route.query.isPreview;

      if (!isPreview) {
        return false;
      }

      if (isPreview === "true" || isPreview === "True") {
        return true;
      }

      return false;
    },

    getEventDescription() {
      const eventID = this.$route.query.eventID;
      const isPreview = this.getIsPreviewParameter();
      const checkCode = this.$route.query.checkCode;
      const lang = this.getLanguageByQueryString();

      eventApi
        .getEventDescription(this, eventID, isPreview, checkCode, lang)
        .then((res) => {
          this.eventDescription = res.data;
        });
    },

    closeAllModal() {
      this.isMaintainMsgRead = true;
    },
  },
};
</script>

<style scoped>
@media only screen and (min-width: 0px) and (max-width: 767px) {
  .event-title {
    font-size: 20px !important;
  }
}
</style>
