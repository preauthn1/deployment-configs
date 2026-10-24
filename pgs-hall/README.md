# target Demo Hall

使用 `gin` 架設的 Web Service，負責以一個 `merchant` 的角度展示該 `merchant` 所開放的遊戲

> 初版的遊戲列表為設定檔寫死，透過  `internal/game` 內的功能來取得遊戲
>
> 使用 `i18n` 來達成多國語系
> 
> 當使用 `vendor` API 來呈現時，這兩個資料夾就廢棄了

* [主要流程](#主要流程)
* [資料結構](#資料結構)
* [CI/CD](#CI/CD)

### 主要流程
1. 透過 `vendor` API 查詢該供應商有哪些遊戲
2. 陳列出所有遊戲供使用者選擇
3. 向 `vendor` API 查詢遊戲連結
   > 若有多個房間則會提供選房間的連結

### 資料結構

* i18n: 多國語系設定檔，已無使用
* internal: 內部邏輯
  * game: 遊戲列表功能
  * vendor: 供應商相關功能
* static: 靜態資源，遊戲圖片
  > 遊戲入口圖會到 `static/game-icon/` 下找`遊戲簡稱`為名的 `.png` 檔案
* views: 要render的html設定

### CI/CD

此 Project 作
* 測試
* 建立執行檔
* 建立Docker Image
* 發動 Deploy (呼叫 [Remote](http://example.invalid:1688/target/deployment/hall))