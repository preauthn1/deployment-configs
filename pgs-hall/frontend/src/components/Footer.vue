<template>
    <div class="footer-bg">
        <div class="id-text">{{username}}</div>
        <div class="balance-text">{{balanceText}}</div>
    </div>
</template>
<script>
export default {
    props: ["username", "balance"],
    computed: {
        balanceText() {
            return this.numberWithCommas(this.balance);
        },
    },
    methods: {
        numberWithCommas(x) {
            return x.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
        },
    },
};
</script>
<style>
.footer-bg {
    background-image: url("../assets/images/footer/footer_bg.png");
    background-size: contain;
    background-repeat: no-repeat;
    background-position: bottom;
    position: fixed;
    left: 50%;
    bottom: -0.5rem;
    transform: translate(-50%, 0);
    width: 400rem;
    height: 12.3rem;
}

.id-text {
    bottom: 72%;
    text-overflow: clip;
}
.balance-text {
    color: #fee94e;
    bottom: 47%;
}

.id-text,
.balance-text {
    text-align: center;
    position: absolute;
    font-size: 15px;
    width: 5.1%;
    left: 50.6%;
    transform: translate(-50.6%, 0);
}

@media only screen and (min-width: 0px) and (max-width: 767px) {
    .footer-bg {
        width: 290rem;
        height: 13rem;
    }
    .id-text,
    .balance-text {
        font-size: 1.6rem;
    }
}

@media only screen and (min-width: 768px) {
    .footer-bg {
        width: 400rem;
        height: 18rem;
    }
    .id-text,
    .balance-text {
        font-size: 2.3rem;
    }
}
</style>