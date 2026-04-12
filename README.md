# telegram-garbage-reminder

寶山鄉雙溪線 Telegram 垃圾車提醒服務

一個以 Go 撰寫的本機 Telegram 提醒服務，固定追蹤 `新竹縣寶山鄉` 的 `雙溪線(每周一、二、四、五資源回收)`，在 `有謙家園` 站點到站前 `10 分鐘` 與 `1 分鐘` 發送提醒。

## 功能

- 使用 Eupfin / 樂圾通 API 驗證目標路線與站點
- 每分鐘檢查一次是否進入提醒時間窗
- Telegram 單一私人 chat 通知
- 啟動後立即發送一封測試提醒
- 提醒訊息內含文字與雙標記地圖連結
- 即時 GPS 優先，查不到時自動退回站點座標
- GPS 查詢冷卻保護，避免過度頻繁打 Eupfin API
- 本機 JSON state 去重，避免重啟後重複提醒

## 固定目標

- `TARGET_CUST_ID=5005808`
- `TARGET_ROUTE_ID=461`
- `TARGET_POINT_SEQ=27`
- `TARGET_POINT_NAME=有謙家園`
- `TARGET_TIME=20:30`
- `TARGET_DAYS=MON,TUE,THU,FRI`
- `REMINDER_MINUTES=10,1`

## 環境變數

請先複製 `.env.example` 為 `.env`，再填入至少兩個值：

```bash
TELEGRAM_BOT_TOKEN=your_telegram_bot_token_here
TELEGRAM_CHAT_ID=your_telegram_chat_id_here
```

其他預設設定：

```bash
EUPFIN_BASE_URL=https://customer-tw.eupfin.com/Eup_Servlet_Nuser_SOAP/Eup_Servlet_Nuser_SOAP
STATE_FILE=data/state.json
CHECK_INTERVAL=1m
GPS_REFRESH_INTERVAL=5m
SEND_TEST_MESSAGE_ON_START=true
```

## 執行方式

```bash
go run cmd/server/main.go
```

若你是在 Windows PowerShell，可以直接用：

```powershell
Copy-Item .env.example .env
# 編輯 .env，填入 TELEGRAM_BOT_TOKEN 與 TELEGRAM_CHAT_ID
.\scripts\run-local.ps1
```

啟動後程式會先做兩件事：

1. 向 Eupfin API 驗證寶山鄉雙溪線的 `有謙家園` 站點是否存在
2. 立即發送一封測試提醒，並載入或建立 `data/state.json`

程式會常駐執行，直到你按 `Ctrl+C`。

## 提醒內容

每次提醒都會包含：

- 路線名稱
- 站點名稱與站序
- 預定到站時間
- 目前提醒時間點
- GPS 模式
- 垃圾車目前座標
- 有謙家園座標
- 一個 Google Maps 雙點地圖連結

若即時 GPS 查不到，訊息會明確標示：

`GPS 暫不可用，已改用有謙家園站點位置。`

## 測試

建議先執行：

```bash
go test ./cmd/... ./internal/...
```

若你只想先確認設定是否齊全，最簡單的手動流程是：

1. 複製 `.env.example` 成 `.env`
2. 填入 `TELEGRAM_BOT_TOKEN` 與 `TELEGRAM_CHAT_ID`
3. 執行 `.\scripts\run-local.ps1`
4. Telegram 先收到一封 `啟動測試提醒`
5. 看到 `Validated target stop` 與 `Reminder scheduler started` 代表啟動成功

目前測試涵蓋：

- 設定解析與必要環境變數檢查
- Eupfin target route / point 驗證
- 啟動測試提醒格式
- 提醒去重
- GPS 冷卻快取
- GPS 與站點 fallback 訊息內容

## 專案結構

```text
cmd/server/          主程式入口
internal/config/     環境變數與提醒目標設定
internal/eupfin/     樂圾通 API client
internal/notifier/   Telegram Bot API 發送
internal/reminder/   提醒排程與訊息組裝
internal/state/      本機 JSON state
```
