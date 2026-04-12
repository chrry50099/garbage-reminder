# Garbage ETA Predictor

這個 Home Assistant App 會在每週一到六晚上收集雙溪線垃圾車的 GPS 與站點狀態，根據歷史資料預測抵達 `有謙家園` 的時間，並在接近時同步發送：

- Telegram 訊息
- Home Assistant TTS 廣播，例如 HomePod Mini

## 安裝

1. 在 Home Assistant `設定 > 系統 > Apps > App Store`
2. 右上角選單加入這個 GitHub repository
3. 安裝 `Garbage ETA Predictor`
4. 在 App 設定頁填入必要參數後啟動

啟動成功後，Home Assistant 側邊欄會出現 `Garbage ETA`，可直接打開查看目前 ETA、GPS、通知狀態與原始 `/status`，也可在頁面裡直接發送 HomePod Mini 測試播報。

## 必填設定

- `telegram_bot_token`
- `telegram_chat_id`
- `ha_notify_mode`
- `ha_tts_target`

Home Assistant API 不需要另外填 URL 或 long-lived token，App 會透過 Supervisor 內部代理直接和 HA 溝通。

## Home Assistant 端要先準備什麼

你仍然需要在 HA 內準備好「收到通知後怎麼播報」的流程。

推薦先用 `webhook` 模式：

- `ha_notify_mode`: `webhook`
- `ha_tts_target`: `garbage_truck_eta`

然後把 `deploy/home_assistant/automation_webhook.yaml` 的內容改成你的實際 `tts.*` 與 `media_player.*` entity 後匯入 HA。

如果你偏好用 script service call，也可以改成：

- `ha_notify_mode`: `service_call`
- `ha_tts_target`: `script.homepod_broadcast`

並搭配 `deploy/home_assistant/script_service_call.yaml`。

## 狀態頁

App 會提供兩種狀態查看方式：

- Home Assistant 側邊欄的 `Garbage ETA` Ingress UI
- `GET /status` JSON API

側邊欄 UI 內建測試播報面板，可：

- 輸入你想播報的文字
- 勾選 1 台或多台 HomePod Mini / media_player
- 選擇 HA 內可用的 TTS entity
- 直接送出測試廣播

如果你選的是 `Google Gemini TTS`，請不要手動填 `language`。Gemini 會自動偵測輸入語言，App 也會自動忽略這個欄位，避免中文播報失敗。

如果某台 HomePod 對特定中文播報回 500，請改用 Gemini voice `achernar` 或 `leda`。目前 App 測試面板預設會使用 `achernar`。

Home Assistant OS 版本預設不直接對外開放 host port，避免和現有服務撞埠造成啟動失敗。

如果你要從外部直接打 `/status`：

1. 到 App 設定頁把 `8080/tcp` 對外埠打開
2. 重新啟動 App
3. 再用 `http://<home-assistant-ip>:<你設定的port>/status`

如果你想把 ETA 顯示到 Home Assistant，可參考 `deploy/home_assistant/rest_status_sensor.yaml`。

## 預設追蹤目標

- 客戶代號：`5005808`
- 路線：`雙溪線`
- 站點順序：`27`
- 站點名稱：`有謙家園`

## 備註

- 第一次安裝後，系統需要先累積 1-2 週資料，歷史預測才會越來越穩定。
- 如果同 weekday 的歷史樣本不足，系統會退回使用 Eupfin API 的估計時間，不會硬做低品質預測。
