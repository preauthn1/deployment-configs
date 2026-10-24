<template>
  <div class="messagesPage underFixHeader pc-max-width">
    <div
      class="text-c"
      style="font-size: 14px; padding: 25px"
      v-if="!isListNotEmpty && isListApiFinish"
    >
      {{ $t("Msg_NoMessages") }}
      <div class="text-c">
        <i
          class="fas fa-envelope"
          style="font-size: 22px; margin-top: 10px; color: #ddd"
        ></i>
      </div>
    </div>

    <div
      style="color: #ccc; font-size: 12px; margin-bottom: 5px"
      v-if="isListNotEmpty"
    >
      <!-- <i class="fas fa-error"></i> -->

      <div style="display: flex; padding: 0px 8px">
        <div class="text-l msg-maxSize" style="flex: 2">
          <i class="fas fa-exclamation-circle"></i>
          {{ $t("Msg_OnlyShowNewstNumberOfSize", { size: "20" }) }}
        </div>
        <div class="text-r" style="flex: 2; color: #aaa">
          {{ $t("Timezone") }}: GMT+{{ displayTimezone }}
        </div>
      </div>
    </div>

    <div
      class="messages"
      v-for="(item, index) in listOfMessages"
      :key="index"
      @click="showPopupMessageDetail(item)"
    >
      <div class="titleRow">
        <div class="title-text">
          <span v-if="!item.isRead">
            <i class="far fa-envelope unread-tag"></i>
          </span>
          <span v-else>
            <i class="far fa-envelope read-tag"></i>
          </span>

          {{ item.title }}
        </div>
        <span class="time"> {{ item.createTimeText }} </span>
      </div>

      <div class="divide-row"></div>

      <div class="contentRow">
        {{ item.content }}
      </div>
    </div>

    <!--   

    <div class="messages">
      <div class="titleRow">
        <div class="title-text">
          <span class="message-type-label">系統</span>
          系統維護12:00-14:00
        </div>
        <span class="time">2021/11/16 14:36:00 </span>
      </div>

      <div class="divide-row"></div>
      <div class="contentRow">
        系統將於 2021/16/01 進行例行維護。造成不便進請見諒
      </div>
    </div> -->

    <div class="modal-overlay" v-if="isShowPopup" @click="closePopup()">
      <div class="popup-modal" style="margin-top: 50px">
        <div class="modal-body">
          <!-- <i
            class="fas fa-times-circle closeButton"
            style="font-size: 20px"
            @click="closeAndSetMessageRead(targetPopup.message)"
          ></i> -->

          <div class="modal-title" style="font-size: 15px; margin-bottom: 5px">
            <div style="color: #fff">
              <span class="message-type-label">
                {{ $t("Label_Tag_System") }}</span
              >
              {{ targetPopup.title }}
            </div>

            <div
              class="text-r"
              style="flex: 2; font-size: 11px; color: #ccc; margin-bottom: 5px"
            >
              {{ targetPopup.createTimeText }}
            </div>
          </div>

          <div
            class="modal-description"
            style="
              color: #ddd;
              padding: 8px;
              border-radius: 5px;
              background: #333;
              min-height: 200px;
              margin-bottom: 0px;
            "
          >
            {{ targetPopup.content }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import commonMixin from "../utilities/mixin/commonMixin";
import announcementApi from "../utilities/api/announcementApi";

export default {
  components: {},

  mixins: [commonMixin],
  computed: {
    isListNotEmpty() {
      return this.listOfMessages && this.listOfMessages.length > 0;
    },
  },

  data() {
    return {
      displayTimezone: 8,
      listOfMessages: [],
      isListApiFinish: false,
      isShowPopup: false,
      targetPopup: {
        title: "title",
        description: "description",
      },
    };
  },

  created() {
    this.getMessageList();
  },

  methods: {
    showPopupMessageDetail(targetItem) {
      this.isShowPopup = true;
      this.targetPopup = targetItem;

      announcementApi.setMessageRead(this, targetItem.messageID).then((res) => {
        console.log(res);
      });
    },

    closePopup() {
      this.isShowPopup = false;
      this.getMessageList();
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

    getMessageList() {
      announcementApi
        .getMessageList(this)
        .then((res) => {
          this.displayTimezone = res.data.timeZone;
          this.listOfMessages = res.data.items;
          this.targetPopup = this.listOfMessages[0];
        })
        .finally(() => {
          this.isListApiFinish = true;
        });
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
