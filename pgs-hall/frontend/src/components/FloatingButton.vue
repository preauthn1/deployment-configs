<template>
    <div id="main-wrapper" class="pointer-events-none">
        <div id="floating-snap-btn-wrapper">
            <div class="fab-container">
                <div class="fab-btn"></div>
                <div
                    ref="fabNum"
                    class="fab-num"
                    :class="{ 'fab-num-animation': animatedNum }"
                >
                    {{ fabNumber }}
                </div>
            </div>
        </div>
    </div>
</template>

<script>
export default {
    props: ["redEnvelopeCount"],
    computed: {
        fabNumber() {
            return this.redEnvelopeCount > 999 ? 999 : this.redEnvelopeCount;
        },
    },
    data() {
        return {
            animatedNum: false,
        };
    },
    watch: {
        redEnvelopeCount: function (newValue, oldValue) {
            if (newValue > oldValue) {
                this.animatedNum = true;
            }
        },
    },
    mounted() {
        this.$refs.fabNum.addEventListener(
            "animationend",
            this.resetNumAnimation
        );
        this.setFloatingButton();
    },

    beforeUnmount() {
        this.$refs.fabNum.removeEventListener(
            "animationend",
            this.resetNumAnimation
        );
    },

    methods: {
        resetNumAnimation() {
            this.animatedNum = false;
        },

        setFloatingButton() {
            const fabElement = document.getElementById(
                "floating-snap-btn-wrapper"
            );
            const wrapperElement = document.getElementById("main-wrapper");
            let oldPositionX, oldPositionY;
            let currPositionX, currPositionY;

            const move = (e) => {
                if (!fabElement.classList.contains("fab-active")) {
                    if (e.type === "touchmove") {
                        example.invalid = e.touches[0].clientY + "px";
                        fabElement.style.left = e.touches[0].clientX + "px";
                    } else {
                        example.invalid = e.clientY + "px";
                        fabElement.style.left = e.clientX + "px";
                    }
                }
            };

            const mouseDown = (e) => {
                // console.log("mouse down");
                oldPositionY = example.invalid;
                oldPositionX = fabElement.style.left;
                if (e.type === "mousedown") {
                    window.addEventListener("mousemove", move);
                } else {
                    window.addEventListener("touchmove", move);
                }

                wrapperElement.classList.add("pointer-events-auto");
                wrapperElement.classList.remove("pointer-events-none");

                fabElement.style.transition = "none";
            };

            const mouseUp = (e) => {
                // console.log("mouse up");
                if (e.type === "mouseup") {
                    window.removeEventListener("mousemove", move);
                } else {
                    window.removeEventListener("touchmove", move);
                }
                wrapperElement.classList.add("pointer-events-none");
                wrapperElement.classList.remove("pointer-events-auto");
                snapToSide(e);
                fabElement.style.transition = "0.3s ease-in-out left";
            };

            const snapToSide = (e) => {
                const windowWidth = window.innerWidth;
                if (e.type === "touchend") {
                    currPositionX = e.changedTouches[0].clientX;
                    currPositionY = e.changedTouches[0].clientY;
                } else if (e.type === "mouseup") {
                    currPositionX = e.clientX;
                    currPositionY = e.clientY;
                }
                if (currPositionY < 50) {
                    example.invalid = 50 + "px";
                }
                if (currPositionY > wrapperElement.clientHeight - 50) {
                    example.invalid =
                        wrapperElement.clientHeight - 50 + "px";
                }
                if (currPositionX < windowWidth / 2) {
                    fabElement.style.left = 40 + "px";
                    fabElement.classList.remove("right");
                    fabElement.classList.add("left");
                } else {
                    fabElement.style.left = windowWidth - 40 + "px";
                    fabElement.classList.remove("left");
                    fabElement.classList.add("right");
                }
            };

            fabElement.addEventListener("mousedown", mouseDown);
            fabElement.addEventListener("mouseup", mouseUp);
            fabElement.addEventListener("touchstart", mouseDown);
            fabElement.addEventListener("touchend", mouseUp);
            fabElement.addEventListener("click", () => {
                if (
                    example.invalid === "" ||
                    (oldPositionY === example.invalid &&
                        oldPositionX === fabElement.style.left)
                ) {
                    this.$emit("button-click");
                }
            });
            window.onresize = function (event) {
                snapToSide(event);
            };
        },
    },
};
</script>

<style lang="scss" scoped>
* {
    padding: 0;
    margin: 0;
    box-sizing: border-box;
}

.pointer-events-none {
    pointer-events: none;
}
.pointer-events-auto {
    pointer-events: auto;
}

#main-wrapper {
    position: absolute;
    left: 0;
    top: 0;
    width: 100%;
    height: 100%;
    background-color: transparent;
    overflow: hidden;

    #floating-snap-btn-wrapper {
        position: absolute;
        transform: translate(-50%, -50%);
        top: 30%;
        right: -10px;
        width: 50px;
        height: 70px;
        cursor: pointer;
    }
    .fab-container {
        animation: bounce-in-btn 0.5s;
        width: 100%;
        height: 100%;
    }
    .fab-btn {
        pointer-events: auto;
        position: absolute;
        display: flex;
        justify-content: center;
        align-items: center;
        top: 0;
        right: 0;
        width: 50px;
        height: 70px;
        background-image: url("../assets/images/event/red_envelope.png");
        background-size: 50px 70px;
        background-repeat: no-repeat;
        color: white;
        box-shadow: 0px 2px 17px -1px rgba(0, 0, 0, 0.3);
    }
    .fab-num {
        position: absolute;
        bottom: -15px;
        left: -15px;
        font-size: 20px;
        line-height: 40px;
        height: 40px;
        width: 40px;
        font-weight: bold;
        background-color: #e49800;
        border-radius: 50%;
    }

    .fab-num-animation {
        animation: bounce-in-num 0.5s;
    }

    @keyframes bounce-in-btn {
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

    @keyframes bounce-in-num {
        0% {
            transform: scale(1);
        }
        50% {
            transform: scale(1.5);
        }
        100% {
            transform: scale(1);
        }
    }
}
</style>