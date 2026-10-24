import { createI18n } from 'vue-i18n' 
import en_us from "./static/lang/en_us.json";
import zh_cn from "./static/lang/zh_cn.json";
import ms_my from "./static/lang/ms_my.json";
import vi_vn from "./static/lang/vi_vn.json";
import th_th from "./static/lang/th_th.json";
import pt_pt from "./static/lang/pt_pt.json";

let defaultLang = localStorage.getItem("LOCALE");

if(!defaultLang || defaultLang == null)
{
  defaultLang = 'en-us';
}

const i18n = createI18n({
    globalInjection: true,
    fallbackLocale: 'en-us',
    locale: defaultLang,
    messages: {
      "en-us": en_us,
      "zh-cn": zh_cn,
      "zh-tw": zh_cn,
      "ms-my": ms_my,
      "vi-vn": vi_vn,
      "th-th": th_th,
      "pt-pt": pt_pt,
    }
  });
  
export default i18n;