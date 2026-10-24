<template>
  <!-- 彈出訊息 -->

  <div class="modal-overlay" v-if="listOfAnnouncement.length > 0">
    <div
      class="popup-modal"
      v-for="(item, index) in listOfAnnouncement"
      v-show="isArrayFirst(item)"
      :key="index"
    >
      <div class="modal-body">
        <img
          src="../assets/images/lobby/close_button.png"
          class="closeButton"
          @click="closeAndSetMessageRead(item.announcementID)"
        />

        <div class="modal-title" v-if="notEmptyNullText(item.title)">
          {{ item.title }}
        </div>

        <div
          class="modal-description"
          v-if="notEmptyNullText(item.description)"
        >
          {{ item.description }}
        </div>

        <div class="content">
          <img
            style="width: 100%"
            :src="item.imageUrlMiddle"
            v-if="notEmptyNullText(item.imageUrlMiddle)"
            :class="item.linkType > 0 ? 'pointer' : ''"
            @click="clickToLinkPage(item)"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import announcementApi from "../utilities/api/announcementApi";
import commonMixin from "../utilities/mixin/commonMixin";

export default {
  mixins: [commonMixin],
  data() {
    return {
      rewardImage: "",
      isShowPopup: false,
      listOfAnnouncement: [],
      closeMessageIDs: [],
    };
  },

  computed: {},

  mounted() {
    this.getUnreadAnnouncement();
  },

  methods: {
    isArrayFirst(item) {
      if (this.listOfAnnouncement.length <= 0) {
        return false;
      }
      return this.listOfAnnouncement[0].announcementID == item.announcementID;
    },

    clickToLinkPage(item) {
      if (item.linkType === 99) {
        window.open(item.linkReference, "_blank");
      } else if (item.linkType === 21) {
        //視為已讀
        this.closeAndSetMessageRead(item.announcementID);

        setTimeout(() => {
          window.location.href =
            window.location.href.split("?")[0] +
            "?page=game&gameID=" +
            item.linkReference;
        }, 500);
      }
    },

    closeAndSetMessageRead(announcementID) {
      announcementApi
        .setAnnouncementRead(this, announcementID)
        .then((res) => {
          console.log(res);

          //移除陣列
          this.listOfAnnouncement = this.listOfAnnouncement.filter(
            (item) => item.announcementID !== announcementID
          );
        })
        .finally(() => {});
    },

    getUnreadAnnouncement() {
      announcementApi
        .getUnreadAnnouncement(this)
        .then((res) => {
          this.listOfAnnouncement = res.data.items;

          if (this.listOfAnnouncement.length > 0) {
            this.isShowPopup = true;
          }
        })
        .finally(() => {});
    },
  },
};
</script>

<style scoped>
.pointer {
  cursor: pointer;
}
</style>