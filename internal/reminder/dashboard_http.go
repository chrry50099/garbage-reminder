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

    .badge strong,
    .meta-pill strong {
      font-weight: 650;
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

    .error {
      color: var(--danger);
    }

    @media (max-width: 900px) {
      .eta-panel,
      .status-panel,
      .wide-panel {
        grid-column: span 12;
      }

      .stats {
        grid-template-columns: 1fr;
      }

      .kv {
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
      </div>
    </section>
  </main>
  <script>
    const statusURL = new URL("./status", window.location.href);
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
      statusLink: document.getElementById("status-link")
    };

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
      if (!lat && !lng) return "--";
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

      if (status.gps_available && status.target_lat && status.target_lng) {
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

    refreshStatus();
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
