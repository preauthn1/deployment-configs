<template>
    <div class="group">
        <div v-for="i in totalCount" :key="i" :class="i == currentIndex ? 'enable': 'disable'"></div>
    </div>
</template>

<script>
export default {
    data() {
        return {
            totalCount: 0,
            currentIndex: 0,
        };
    },
    methods: {
        handleScroll() {
            this.totalCount = Math.ceil(
                document.body.offsetHeight / window.innerHeight
            );
            if (this.totalCount > 9) {
                this.totalCount = 9;
            }
            let normalized =
                window.scrollY /
                (document.body.offsetHeight - window.innerHeight);
            let index = Math.ceil(this.totalCount * normalized);
            if (index > this.totalCount) index = this.totalCount;
            if (index <= 0) index = 1;
            this.currentIndex = index;
        },
    },
    created() {
        window.addEventListener("scroll", this.handleScroll);
    },
    mounted() {
        this.handleScroll();
    },
    unmounted() {
        window.removeEventListener("scroll", this.handleScroll);
    },
};
</script>

<style scoped>
.group {
    position: fixed;
    /* grid-gap: 1rem; */
    display: grid;
    grid-template-columns: auto;
    left: 1.5rem;
    top: 50%;
    transform: translate(0, -50%);
}
.enable,
.disable {
    background-repeat: no-repeat;
    background-position: center;
    background-size: contain;
    height: 3rem;
    width: 3rem;
}
.enable {
    background-image: url("../assets/images/lobby/toggle_enable.png");
}
.disable {
    background-image: url("../assets/images/lobby/toggle_disable.png");
}
</style>