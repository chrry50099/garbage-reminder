package reminder

import (
	"html/template"
	"net/http"
)

var dashboardTemplate = template.Must(template.New("dashboard").Parse(`<!DOCTYPE html>
<html lang="zh-Hant">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Garbage ETA Predictor</title>
  <style>
    :root {
      color-scheme: dark;
      --bg: radial-gradient(circle at top, #205c53 0%, #11222a 45%, #081116 100%);
      --panel: rgba(8, 17, 22, 0.78);
      --panel-border: rgba(197, 255, 233, 0.15);
      --accent: #8ef0d0;
      --accent-strong: #f7d46f;
      --text: #f5f9f7;
      --muted: #b8c7c1;
      --danger: #ff8e8e;
      --success: #9ef3b5;
      --shadow: 0 24px 60px rgba(0, 0, 0, 0.25);
      --radius: 24px;
      --font: "IBM Plex Sans", "Noto Sans TC", "Segoe UI", sans-serif;
    }

    * {
      box-sizing: border-box;
    }

    body {
      margin: 0;
      min-height: 100vh;
      background: var(--bg);
      color: var(--text);
      font-family: var(--font);
    }

    .shell {
      max-width: 1180px;
      margin: 0 auto;
      padding: 24px;
    }

    .hero,
    .panel {
      background: var(--panel);
      border: 1px solid var(--panel-border);
      box-shadow: var(--shadow);
      backdrop-filter: blur(18px);
      border-radius: var(--radius);
    }

    .hero {
      padding: 28px;
      position: relative;
      overflow: hidden;
    }

    .hero::after {
      content: "";
      position: absolute;
      inset: auto -80px -100px auto;
      width: 260px;
      height: 260px;
      border-radius: 50%;
      background: radial-gradient(circle, rgba(247, 212, 111, 0.32) 0%, rgba(247, 212, 111, 0) 70%);
      pointer-events: none;
    }

    .eyebrow {
      font-size: 0.82rem;
      letter-spacing: 0.18em;
      text-transform: uppercase;
      color: var(--accent);
      margin: 0 0 12px;
    }

    .title-row {
      display: flex;
      justify-content: space-between;
      gap: 16px;
      align-items: flex-start;
      flex-wrap: wrap;
    }

    h1 {
      margin: 0;
      font-size: clamp(1.8rem, 4vw, 3rem);
      line-height: 1.05;
      font-weight: 650;
    }

    .subtitle {
      margin: 8px 0 0;
      color: var(--muted);
      font-size: 1rem;
    }

    .badge-row,
    .meta-row {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
    }

    .badge,
    .meta-pill {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      border-radius: 999px;
      padding: 9px 14px;
      background: rgba(255, 255, 255, 0.06);
      color: var(--text);
      font-size: 0.92rem;
    }

    .badge.active {
      background: rgba(142, 240, 208, 0.14);
      color: var(--accent);
    }

    .badge.inactive {
      background: rgba(255, 142, 142, 0.12);
      color: var(--danger);
    }

    .grid {
      display: grid;
      grid-template-columns: repeat(12, minmax(0, 1fr));
      gap: 18px;
      margin-top: 18px;
    }

    .panel {
      padding: 22px;
    }

    .panel h2 {
      margin: 0 0 16px;
      font-size: 1.05rem;
      font-weight: 620;
      letter-spacing: 0.02em;
    }

    .eta-panel {
      grid-column: span 7;
      display: grid;
      gap: 16px;
    }

    .status-panel {
      grid-column: span 5;
      display: grid;
      gap: 16px;
    }

    .wide-panel {
      grid-column: span 6;
    }

    .full-panel {
      grid-column: span 12;
    }

    .stats {
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 14px;
    }

    .stat-card {
      padding: 18px;
      border-radius: 20px;
      background: rgba(255, 255, 255, 0.05);
      border: 1px solid rgba(255, 255, 255, 0.05);
    }

    .stat-label {
      color: var(--muted);
      font-size: 0.82rem;
      margin-bottom: 8px;
    }

    .stat-value {
      font-size: clamp(1.1rem, 2.2vw, 1.6rem);
      font-weight: 620;
    }

    .hero-value {
      font-size: clamp(2.4rem, 7vw, 4.6rem);
      line-height: 0.95;
      font-weight: 680;
      margin: 2px 0 6px;
    }

    .muted {
      color: var(--muted);
    }

    .message {
      margin-top: 10px;
      padding: 14px 16px;
      border-radius: 18px;
      background: rgba(255, 255, 255, 0.05);
      color: var(--muted);
      line-height: 1.5;
    }

    .message.success {
      color: var(--success);
      background: rgba(158, 243, 181, 0.08);
    }

    .message.error {
      color: var(--danger);
      background: rgba(255, 142, 142, 0.08);
    }

    dl {
      margin: 0;
      display: grid;
      gap: 12px;
    }

    .kv {
      display: grid;
      grid-template-columns: 120px 1fr;
      gap: 12px;
      align-items: start;
    }

    dt {
      color: var(--muted);
    }

    dd {
      margin: 0;
      font-weight: 510;
      word-break: break-word;
    }

    a {
      color: var(--accent);
    }

    pre {
      margin: 0;
      white-space: pre-wrap;
      word-break: break-word;
      border-radius: 18px;
      background: rgba(0, 0, 0, 0.26);
      padding: 16px;
      font-size: 0.84rem;
      line-height: 1.5;
      color: #d8f2e9;
    }

    .form-grid {
      display: grid;
      grid-template-columns: repeat(12, minmax(0, 1fr));
      gap: 14px;
    }

    .field {
      grid-column: span 12;
      display: grid;
      gap: 8px;
    }

    .field.half {
      grid-column: span 6;
    }

    label {
      font-size: 0.92rem;
      color: var(--muted);
    }

    textarea,
    select,
    input {
      width: 100%;
      border: 1px solid rgba(255, 255, 255, 0.12);
      border-radius: 16px;
      background: rgba(0, 0, 0, 0.22);
      color: var(--text);
      padding: 14px 16px;
      font: inherit;
    }

    textarea {
      min-height: 120px;
      resize: vertical;
    }

    .device-list {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
      gap: 10px;
    }

    .device-option {
      display: flex;
      gap: 12px;
      align-items: flex-start;
      padding: 14px;
      border-radius: 18px;
      background: rgba(255, 255, 255, 0.05);
      border: 1px solid rgba(255, 255, 255, 0.08);
    }

    .device-option input[type="checkbox"] {
      width: auto;
      margin-top: 4px;
      accent-color: #8ef0d0;
    }

    .device-meta strong {
      display: block;
      margin-bottom: 6px;
      font-size: 0.98rem;
    }

    .device-meta span {
      color: var(--muted);
      font-size: 0.86rem;
    }

    .actions {
      display: flex;
      gap: 12px;
      align-items: center;
      flex-wrap: wrap;
    }

    button {
      border: 0;
      border-radius: 999px;
      padding: 12px 18px;
      background: linear-gradient(135deg, #8ef0d0 0%, #f7d46f 100%);
      color: #081116;
      font: inherit;
      font-weight: 700;
      cursor: pointer;
    }

    button:disabled {
      cursor: not-allowed;
      opacity: 0.55;
    }

    .helper {
      color: var(--muted);
      font-size: 0.86rem;
      line-height: 1.5;
    }

    @media (max-width: 900px) {
      .eta-panel,
      .status-panel,
      .wide-panel,
      .full-panel {
        grid-column: span 12;
      }

      .stats {
        grid-template-columns: 1fr;
      }

      .kv,
      .field.half {
        grid-column: span 12;
        grid-template-columns: 1fr;
        gap: 6px;
      }
    }
  </style>
</head>
<body>
  <main class="shell">
    <section class="hero">
      <p class="eyebrow">Home Assistant App</p>
      <div class="title-row">
        <div>
          <h1 id="route-title">Garbage ETA Predictor</h1>
          <p class="subtitle" id="route-subtitle">雙溪線 / 有謙家園</p>
        </div>
        <div class="badge-row">
          <span class="badge" id="service-badge">載入中</span>
          <span class="badge" id="prediction-badge">等待資料</span>
        </div>
      </div>
      <div class="grid">
        <div class="panel eta-panel">
          <div>
            <div class="muted">預測剩餘時間</div>
            <div class="hero-value" id="remaining-minutes">--</div>
            <div class="muted" id="arrival-at">尚未取得到站預測</div>
          </div>
          <div class="stats">
            <div class="stat-card">
              <div class="stat-label">資料來源</div>
              <div class="stat-value" id="prediction-source">待命中</div>
            </div>
            <div class="stat-card">
              <div class="stat-label">信心等級</div>
              <div class="stat-value" id="prediction-confidence">--</div>
            </div>
            <div class="stat-card">
              <div class="stat-label">歷史比對</div>
              <div class="stat-value" id="prediction-match">--</div>
            </div>
          </div>
          <div class="message" id="status-message">正在讀取目前狀態...</div>
        </div>
        <div class="panel status-panel">
          <h2>今晚狀態</h2>
          <div class="meta-row">
            <span class="meta-pill"><strong id="service-date">--</strong></span>
            <span class="meta-pill"><strong id="weekday">--</strong></span>
          </div>
          <dl>
            <div class="kv"><dt>收集時窗</dt><dd id="collection-window">--</dd></div>
            <div class="kv"><dt>Run 狀態</dt><dd id="run-status">--</dd></div>
            <div class="kv"><dt>最近收集</dt><dd id="last-collected">--</dd></div>
            <div class="kv"><dt>已送提醒</dt><dd id="notified-offsets">--</dd></div>
            <div class="kv"><dt>最後更新</dt><dd id="updated-at">--</dd></div>
          </dl>
        </div>
        <section class="panel wide-panel">
          <h2>車輛與站點</h2>
          <dl>
            <div class="kv"><dt>GPS 狀態</dt><dd id="gps-status">--</dd></div>
            <div class="kv"><dt>垃圾車座標</dt><dd id="truck-coords">--</dd></div>
            <div class="kv"><dt>目標站點</dt><dd id="target-coords">--</dd></div>
            <div class="kv"><dt>地圖</dt><dd><a id="map-link" href="#" target="_blank" rel="noreferrer">開啟雙點地圖</a></dd></div>
          </dl>
        </section>
        <section class="panel wide-panel">
          <h2>API 與除錯</h2>
          <dl>
            <div class="kv"><dt>EstimatedTime</dt><dd id="api-estimated">--</dd></div>
            <div class="kv"><dt>WaitingTime</dt><dd id="api-waiting">--</dd></div>
            <div class="kv"><dt>狀態 API</dt><dd><a id="status-link" href="./status" target="_blank" rel="noreferrer">開啟 /status JSON</a></dd></div>
          </dl>
          <pre id="raw-json">{
  "message": "loading"
}</pre>
        </section>
        <section class="panel full-panel">
          <h2>HomePod Mini 測試播報</h2>
          <div class="form-grid">
            <div class="field">
              <label for="broadcast-message">播報訊息</label>
              <textarea id="broadcast-message" placeholder="例如：垃圾車測試廣播，請準備倒垃圾。"></textarea>
            </div>
            <div class="field half">
              <label for="tts-entity">TTS 引擎</label>
              <select id="tts-entity"></select>
            </div>
            <div class="field half">
              <label for="tts-language">語言代碼（可留空）</label>
              <input id="tts-language" type="text" placeholder="例如：zh-TW 或 en">
            </div>
            <div class="field">
              <label>選擇要播報的 HomePod mini</label>
              <div class="device-list" id="device-list">
                <div class="helper">正在讀取可用裝置...</div>
              </div>
            </div>
            <div class="field">
              <div class="actions">
                <button id="broadcast-button" type="button" disabled>送出測試播報</button>
                <span class="helper" id="broadcast-summary">請先輸入訊息並勾選至少一台 HomePod。</span>
              </div>
              <div class="message" id="broadcast-result">這裡會顯示送出結果。</div>
            </div>
          </div>
        </section>
      </div>
    </section>
  </main>
  <script>
    const statusURL = new URL("./status", window.location.href);
    const broadcastOptionsURL = new URL("./api/broadcast/options", window.location.href);
    const broadcastTestURL = new URL("./api/broadcast/test", window.location.href);
    const els = {
      routeTitle: document.getElementById("route-title"),
      routeSubtitle: document.getElementById("route-subtitle"),
      serviceBadge: document.getElementById("service-badge"),
      predictionBadge: document.getElementById("prediction-badge"),
      remainingMinutes: document.getElementById("remaining-minutes"),
      arrivalAt: document.getElementById("arrival-at"),
      predictionSource: document.getElementById("prediction-source"),
      predictionConfidence: document.getElementById("prediction-confidence"),
      predictionMatch: document.getElementById("prediction-match"),
      statusMessage: document.getElementById("status-message"),
      serviceDate: document.getElementById("service-date"),
      weekday: document.getElementById("weekday"),
      collectionWindow: document.getElementById("collection-window"),
      runStatus: document.getElementById("run-status"),
      lastCollected: document.getElementById("last-collected"),
      notifiedOffsets: document.getElementById("notified-offsets"),
      updatedAt: document.getElementById("updated-at"),
      gpsStatus: document.getElementById("gps-status"),
      truckCoords: document.getElementById("truck-coords"),
      targetCoords: document.getElementById("target-coords"),
      mapLink: document.getElementById("map-link"),
      apiEstimated: document.getElementById("api-estimated"),
      apiWaiting: document.getElementById("api-waiting"),
      rawJSON: document.getElementById("raw-json"),
      statusLink: document.getElementById("status-link"),
      broadcastMessage: document.getElementById("broadcast-message"),
      ttsEntity: document.getElementById("tts-entity"),
      ttsLanguage: document.getElementById("tts-language"),
      deviceList: document.getElementById("device-list"),
      broadcastButton: document.getElementById("broadcast-button"),
      broadcastSummary: document.getElementById("broadcast-summary"),
      broadcastResult: document.getElementById("broadcast-result")
    };

    let broadcastOptions = { media_players: [], tts_entities: [], default_tts_entity: "" };

    els.statusLink.href = statusURL.toString();

    function formatDateTime(value) {
      if (!value) return "--";
      const date = new Date(value);
      if (Number.isNaN(date.getTime())) return value;
      return new Intl.DateTimeFormat("zh-TW", {
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit"
      }).format(date);
    }

    function formatCoords(lat, lng) {
      if (typeof lat !== "number" || typeof lng !== "number") return "--";
      return lat.toFixed(6) + ", " + lng.toFixed(6);
    }

    function mapURL(truckLat, truckLng, targetLat, targetLng) {
      const params = new URLSearchParams({
        api: "1",
        origin: truckLat + "," + truckLng,
        destination: targetLat + "," + targetLng,
        travelmode: "driving"
      });
      return "https://www.google.com/maps/dir/?" + params.toString();
    }

    function sourceLabel(source) {
      const labels = {
        historical_model: "歷史模型",
        api_estimated_time: "API EstimatedTime",
        api_waiting_time: "API WaitingTime"
      };
      return labels[source] || source || "待命中";
    }

    function confidenceLabel(confidence) {
      const labels = {
        high: "高",
        medium: "中",
        low: "低"
      };
      return labels[confidence] || confidence || "--";
    }

    function selectedTargets() {
      return Array.from(document.querySelectorAll("input[name='broadcast-target']:checked")).map((node) => node.value);
    }

    function updateBroadcastButtonState() {
      const hasMessage = els.broadcastMessage.value.trim().length > 0;
      const targets = selectedTargets();
      els.broadcastButton.disabled = !(hasMessage && targets.length > 0 && els.ttsEntity.value);
      if (!hasMessage) {
        els.broadcastSummary.textContent = "請先輸入測試播報內容。";
      } else if (targets.length === 0) {
        els.broadcastSummary.textContent = "請至少勾選一台要播報的 HomePod。";
      } else if (!els.ttsEntity.value) {
        els.broadcastSummary.textContent = "目前找不到可用的 TTS 引擎。";
      } else {
        els.broadcastSummary.textContent = "將送到 " + targets.length + " 台裝置。";
      }
    }

    function renderBroadcastOptions(options) {
      broadcastOptions = options || { media_players: [], tts_entities: [], default_tts_entity: "" };

      els.ttsEntity.innerHTML = "";
      if ((broadcastOptions.tts_entities || []).length === 0) {
        const option = document.createElement("option");
        option.textContent = "找不到可用 TTS";
        option.value = "";
        els.ttsEntity.appendChild(option);
      } else {
        broadcastOptions.tts_entities.forEach((entity) => {
          const option = document.createElement("option");
          option.value = entity.entity_id;
          option.textContent = entity.friendly_name + " (" + entity.entity_id + ")";
          if (entity.entity_id === broadcastOptions.default_tts_entity) {
            option.selected = true;
          }
          els.ttsEntity.appendChild(option);
        });
      }

      els.deviceList.innerHTML = "";
      if ((broadcastOptions.media_players || []).length === 0) {
        els.deviceList.innerHTML = "<div class='helper'>目前找不到可用的 HomePod 或 media_player。</div>";
      } else {
        broadcastOptions.media_players.forEach((device, index) => {
          const label = document.createElement("label");
          label.className = "device-option";

          const input = document.createElement("input");
          input.type = "checkbox";
          input.name = "broadcast-target";
          input.value = device.entity_id;
          input.checked = index === 0;
          input.addEventListener("change", updateBroadcastButtonState);

          const meta = document.createElement("div");
          meta.className = "device-meta";
          meta.innerHTML = "<strong>" + device.friendly_name + "</strong><span>" + device.entity_id + " / " + device.state + "</span>";

          label.appendChild(input);
          label.appendChild(meta);
          els.deviceList.appendChild(label);
        });
      }

      updateBroadcastButtonState();
    }

    function render(status) {
      const routeName = status.route_name || "雙溪線";
      const pointName = status.point_name || "有謙家園";
      const pointSeq = status.point_seq ? "第 " + status.point_seq + " 站" : "目標站點";

      els.statusMessage.className = "message";
      els.routeTitle.textContent = routeName;
      els.routeSubtitle.textContent = pointName + " / " + pointSeq;
      els.serviceBadge.textContent = status.active ? "收集中" : "待命中";
      els.serviceBadge.className = "badge " + (status.active ? "active" : "inactive");

      const prediction = status.prediction;
      if (prediction) {
        els.predictionBadge.textContent = sourceLabel(prediction.source);
        els.predictionBadge.className = "badge active";
        els.remainingMinutes.textContent = prediction.remaining_minutes + " min";
        els.arrivalAt.textContent = "預測到站 " + formatDateTime(prediction.predicted_arrival_at);
        els.predictionSource.textContent = sourceLabel(prediction.source);
        els.predictionConfidence.textContent = confidenceLabel(prediction.confidence);
        if (prediction.matched_samples || prediction.historical_runs) {
          els.predictionMatch.textContent = prediction.matched_samples + " / " + prediction.historical_runs;
        } else {
          els.predictionMatch.textContent = "--";
        }
      } else {
        els.predictionBadge.textContent = "等待資料";
        els.predictionBadge.className = "badge inactive";
        els.remainingMinutes.textContent = "--";
        els.arrivalAt.textContent = "尚未取得到站預測";
        els.predictionSource.textContent = "待命中";
        els.predictionConfidence.textContent = "--";
        els.predictionMatch.textContent = "--";
      }

      els.statusMessage.textContent = status.message || "服務已啟動，等待下一次輪詢。";
      els.serviceDate.textContent = status.service_date || "--";
      els.weekday.textContent = status.weekday || "--";
      els.collectionWindow.textContent = status.collection_window || "--";
      els.runStatus.textContent = status.run_status || "--";
      els.lastCollected.textContent = formatDateTime(status.last_collected_at);
      els.updatedAt.textContent = formatDateTime(status.updated_at);
      els.notifiedOffsets.textContent = (status.notified_offsets || []).length
        ? status.notified_offsets.map((offset) => offset + " 分鐘").join(" / ")
        : "尚未送出";

      els.gpsStatus.textContent = status.gps_available ? "可用" : "目前不可用";
      els.truckCoords.textContent = status.gps_available ? formatCoords(status.truck_lat, status.truck_lng) : "--";
      els.targetCoords.textContent = formatCoords(status.target_lat, status.target_lng);

      if (status.gps_available && typeof status.target_lat === "number" && typeof status.target_lng === "number") {
        els.mapLink.href = mapURL(status.truck_lat, status.truck_lng, status.target_lat, status.target_lng);
        els.mapLink.textContent = "開啟垃圾車與站點路線";
      } else {
        els.mapLink.href = "#";
        els.mapLink.textContent = "等待 GPS 後可產生地圖";
      }

      els.apiEstimated.textContent = status.api_estimated_text || "--";
      els.apiWaiting.textContent = status.api_waiting_time === null || status.api_waiting_time === undefined
        ? "--"
        : String(status.api_waiting_time);
      els.rawJSON.textContent = JSON.stringify(status, null, 2);
    }

    async function refreshStatus() {
      try {
        const response = await fetch(statusURL, { cache: "no-store" });
        if (!response.ok) {
          throw new Error("HTTP " + response.status);
        }
        const payload = await response.json();
        render(payload);
      } catch (error) {
        els.statusMessage.textContent = "狀態讀取失敗：" + error.message;
        els.statusMessage.className = "message error";
      }
    }

    async function loadBroadcastOptions() {
      try {
        const response = await fetch(broadcastOptionsURL, { cache: "no-store" });
        if (!response.ok) {
          throw new Error("HTTP " + response.status);
        }
        const payload = await response.json();
        renderBroadcastOptions(payload);
        els.broadcastResult.textContent = "已載入可用的播報裝置與 TTS 引擎。";
        els.broadcastResult.className = "message";
      } catch (error) {
        els.deviceList.innerHTML = "<div class='helper'>裝置清單讀取失敗：" + error.message + "</div>";
        els.broadcastResult.textContent = "無法讀取播報設定：" + error.message;
        els.broadcastResult.className = "message error";
        updateBroadcastButtonState();
      }
    }

    async function submitBroadcast() {
      const payload = {
        message: els.broadcastMessage.value.trim(),
        target_entity_ids: selectedTargets(),
        tts_entity_id: els.ttsEntity.value,
        language: els.ttsLanguage.value.trim()
      };

      els.broadcastButton.disabled = true;
      els.broadcastResult.textContent = "正在送出測試播報...";
      els.broadcastResult.className = "message";

      try {
        const response = await fetch(broadcastTestURL, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload)
        });
        const text = await response.text();
        if (!response.ok) {
          throw new Error(text || ("HTTP " + response.status));
        }
        els.broadcastResult.textContent = "已送出測試播報到：" + payload.target_entity_ids.join(" / ");
        els.broadcastResult.className = "message success";
      } catch (error) {
        els.broadcastResult.textContent = "播報送出失敗：" + error.message;
        els.broadcastResult.className = "message error";
      } finally {
        updateBroadcastButtonState();
      }
    }

    els.broadcastMessage.addEventListener("input", updateBroadcastButtonState);
    els.ttsEntity.addEventListener("change", updateBroadcastButtonState);
    els.broadcastButton.addEventListener("click", submitBroadcast);

    refreshStatus();
    loadBroadcastOptions();
    window.setInterval(refreshStatus, 30000);
  </script>
</body>
</html>`))

func NewDashboardHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = dashboardTemplate.Execute(w, nil)
	})
}
