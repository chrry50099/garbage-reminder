# telegram-garbage-reminder

Home Assistant / Telegram 垃圾車動態預測服務

這個服務會固定追蹤 `新竹縣寶山鄉` 的 `雙溪線(每周一、二、四、五資源回收)`，以 `有謙家園` 為目標站點，在每週一到六 `19:00-21:30` 收集垃圾車 GPS 與站點狀態，根據同 weekday 的歷史資料預測到站時間，並在接近時同步發送：

- Telegram 訊息
- Home Assistant 內的 HomePod Mini TTS 廣播
- Home Assistant 側邊欄可直接打開的 ETA 狀態頁
- 側邊欄內建 HomePod Mini 測試播報工具

## 推薦安裝方式

如果你已經是 **Home Assistant OS**，現在可以直接把這個 repo 當成 **Home Assistant App repository** 使用。

關鍵檔案：

- `repository.yaml`
- `config.yaml`
- `run.sh`
- `DOCS.md`

Home Assistant OS 版本安裝說明請看 `DOCS.md`。

如果你不是用 HA OS，也可以繼續用原本的獨立容器方式部署。

## 功能

- 每分鐘收集雙溪線車輛 GPS 與目標站點狀態
- 用 SQLite 保存最近 8 週歷史資料
- 依週一到週六分開使用歷史資料做 ETA 預測
- GPS 歷史樣本不足時，自動退回 Eupfin API EstimatedTime / WaitingTime
- 在 `10` 分鐘與 `3` 分鐘門檻發送雙通道提醒
- 本機 JSON state 去重，避免重啟後重複通知
- 提供 `/status` 只讀端點，方便 Home Assistant REST sensor 或手動檢查
- Home Assistant OS App 版本提供 Ingress UI，可從 HA 側邊欄直接查看 ETA、GPS、通知狀態
- 可在 App UI 輸入任意測試訊息、勾選要播報的 HomePod Mini，直接觸發 HA TTS
- 針對 Google Gemini TTS 自動避開 `language` 參數，修正中文播報失敗

Home Assistant OS App 版本預設不綁定固定 host port，避免和樹梅派上其他服務撞埠。

## 獨立容器模式的主要環境變數

請先複製 `.env.example` 為 `.env`。

必要設定：

```bash
TELEGRAM_BOT_TOKEN=...
TELEGRAM_CHAT_ID=...
HA_BASE_URL=http://homeassistant.local:8123
HA_TOKEN=...
HA_NOTIFY_MODE=webhook
HA_TTS_TARGET=garbage_truck_eta
```

預設目標設定：

```bash
TARGET_CUST_ID=5005808
TARGET_ROUTE_ID=461
TARGET_POINT_SEQ=27
TARGET_POINT_NAME=有謙家園
TARGET_DAYS=MON,TUE,WED,THU,FRI,SAT
ALERT_OFFSETS=10,3
COLLECTION_START=19:00
COLLECTION_END=21:30
HISTORY_WEEKS=8
ARRIVAL_RADIUS_METERS=80
MATCH_RADIUS_METERS=250
MIN_HISTORY_RUNS=3
STATE_FILE=data/state.json
DATABASE_FILE=data/history.db
CHECK_INTERVAL=1m
PORT=8080
```

`HA_NOTIFY_MODE` 支援兩種：

- `webhook`: 呼叫 `POST /api/webhook/<HA_TTS_TARGET>`
- `service_call`: 呼叫 `POST /api/services/<domain>/<service>`，此時 `HA_TTS_TARGET` 要填 `domain.service`

## 獨立容器模式執行方式

本機直接執行：

```bash
go run cmd/server/main.go
```

啟動後服務會：

1. 驗證雙溪線與有謙家園站點
2. 建立 / 開啟 `data/history.db`
3. 啟動每分鐘收集與預測流程
4. 啟動 `GET /status` 狀態端點

## /status

`GET /status` 會回傳目前服務狀態，例如：

- 是否在收集視窗內
- 今天是否已完成該趟 run
- 最新 GPS 與 API 狀態
- 最新預測到站時間、剩餘分鐘、資料來源、信心
- 今天哪些提醒門檻已經送出

## Home Assistant 側邊欄 UI

Home Assistant OS App 版本會自動在側邊欄顯示 `Garbage ETA`，打開後可看到：

- 目前是否在收集視窗內
- 最新 ETA 與預測來源
- 垃圾車 GPS / 目標站點座標
- API EstimatedTime / WaitingTime
- 當天已送出的提醒門檻
- 原始 `/status` JSON 方便除錯
- HomePod Mini 測試播報面板，可選多台裝置與 TTS 引擎

## Docker / Raspberry Pi

Dockerfile 現在同時支援：

- Home Assistant OS App 本機建置
- 一般 Docker / Raspberry Pi 獨立容器建置

範例：

```bash
docker buildx build --platform linux/arm64 -t garbage-reminder:latest .
```

## 測試

```bash
go test ./cmd/... ./internal/...
```

目前測試涵蓋：

- 設定解析與 HA 相關必要欄位
- Eupfin target route / point 驗證
- Home Assistant webhook / service call notifier
- 歷史樣本預測與 API fallback
- 收集視窗判斷、到站完成、通知去重
