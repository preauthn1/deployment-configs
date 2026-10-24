import { createApp } from 'vue'
import App from './App.vue'
import router from './router.js';
import store from './store/index.js';
import Loader from './components/Loader.vue';
import i18n from "./i18nPlugin";
import "@/assets/css/layout.css";
import "bootstrap/dist/css/bootstrap.min.css";
import "bootstrap/dist/js/bootstrap.js";
import "bootstrap/dist/js/bootstrap.bundle.min.js";
import '@fortawesome/fontawesome-free/css/all.css';
import 'video.js/dist/video-js.css' 

const app = createApp(App);
app.use(router);
app.use(store);
app.use(i18n);

app.component('Loader', Loader);
app.mount('#app');