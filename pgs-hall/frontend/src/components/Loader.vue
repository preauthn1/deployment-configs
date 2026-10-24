<template>
  <div class="bg">
    <div class="loader-container">
      <div class="loader"></div>
    </div>
  </div>
</template>

<style scoped>
.bg {
  z-index: 9999;
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
}

/* Safari */
@-webkit-keyframes spin {
  0% {
    -webkit-transform: rotate(0deg);
  }
  100% {
    -webkit-transform: rotate(360deg);
  }
}

@keyframes spin {
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(360deg);
  }
}

.loader-container {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
}
.loader {
  border: 0.5rem solid #f3f3f3;
  border-radius: 50%;
  border-top: 0.5rem solid #3498db;
  width: 3rem;
  height: 3rem;
  -webkit-animation: spin 2s linear infinite; /*  Safari */
  animation: spin 2s linear infinite;
}
</style>