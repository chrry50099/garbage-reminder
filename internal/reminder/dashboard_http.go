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
  <link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css" crossorigin="">
  <style>
    :root{color-scheme:dark;--bg:#0a1216;--panel:#112028;--line:#274048;--text:#f3faf8;--muted:#a8c0b7;--accent:#95f2d2;--warn:#ffd36b;--danger:#ff8f8f;--radius:22px;font-family:"IBM Plex Sans","Noto Sans TC","Segoe UI",sans-serif}
    *{box-sizing:border-box}body{margin:0;background:radial-gradient(circle at top,rgba(61,132,115,.28),transparent 30%),var(--bg);color:var(--text);font-family:var(--font)}
    .shell{max-width:1320px;margin:0 auto;padding:24px}.panel{background:rgba(17,32,40,.9);border:1px solid var(--line);border-radius:var(--radius);padding:20px}
    .hero{padding:28px}.grid{display:grid;grid-template-columns:repeat(12,minmax(0,1fr));gap:18px;margin-top:18px}.s12{grid-column:span 12}.s7{grid-column:span 7}.s5{grid-column:span 5}.s6{grid-column:span 6}
    .title{display:flex;justify-content:space-between;gap:12px;align-items:flex-start;flex-wrap:wrap}.eyebrow{font-size:.82rem;letter-spacing:.18em;text-transform:uppercase;color:var(--accent);margin:0 0 10px}
    h1,h2{margin:0}h1{font-size:clamp(2rem,4vw,3.2rem);line-height:1.03}h2{font-size:1.05rem;margin-bottom:14px}.subtitle,.muted,.helper,label,th,dt{color:var(--muted)}
    .badge{display:inline-flex;padding:10px 14px;border-radius:999px;background:rgba(255,255,255,.06)}.active{color:var(--accent)}.inactive{color:var(--danger)}
    .hero-value{font-size:clamp(2.5rem,7vw,4.8rem);font-weight:700;line-height:.94;margin:8px 0}.stats{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px;margin-top:18px}
    .stat,.card{padding:15px;border-radius:18px;background:rgba(255,255,255,.04);border:1px solid rgba(255,255,255,.06)}.stat-label{font-size:.82rem;color:var(--muted);margin-bottom:8px}.stat-value{font-size:1.3rem;font-weight:620}
    .message{margin-top:12px;padding:14px 16px;border-radius:18px;background:rgba(255,255,255,.05);color:var(--muted)}.error{color:var(--danger)}.success{color:#a4f4bc}
    .kv{display:grid;grid-template-columns:130px 1fr;gap:12px;margin:10px 0}dd{margin:0;word-break:break-word}
    .toolbar,.row,.actions{display:flex;gap:12px;flex-wrap:wrap;align-items:center}.toolbar{justify-content:space-between;margin-bottom:14px}
    select,input,textarea,button{font:inherit;border-radius:14px;border:1px solid rgba(255,255,255,.12);background:rgba(0,0,0,.22);color:var(--text);padding:12px 14px}
    select,input{min-height:46px}textarea{width:100%;min-height:112px;resize:vertical}button{border:0;background:linear-gradient(135deg,var(--accent),var(--warn));color:#071014;font-weight:700;cursor:pointer}button:disabled{opacity:.55;cursor:not-allowed}
    .history{display:grid;grid-template-columns:minmax(320px,.95fr) minmax(0,1.45fr);gap:18px}.summary{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}
    #history-map{min-height:420px;border-radius:18px;overflow:hidden;border:1px solid rgba(255,255,255,.08)}
    .table-wrap{overflow:auto;max-height:540px;border-radius:18px;border:1px solid rgba(255,255,255,.06);background:rgba(0,0,0,.16);margin-top:18px}table{width:100%;border-collapse:collapse;font-size:.9rem}th,td{padding:10px 12px;border-bottom:1px solid rgba(255,255,255,.08);text-align:left;vertical-align:top}thead th{position:sticky;top:0;background:#13222b;z-index:1}
    pre{margin:0;padding:16px;border-radius:18px;background:rgba(0,0,0,.24);overflow:auto;max-height:320px;color:#d8f2e9;font-size:.84rem;line-height:1.5}
    .device-list{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:10px}.device{display:flex;gap:12px;align-items:flex-start;padding:14px;border-radius:18px;background:rgba(255,255,255,.05);border:1px solid rgba(255,255,255,.08)}
    .device input{width:auto;margin-top:4px;accent-color:var(--accent)}
    @media (max-width:980px){.s12,.s7,.s5,.s6{grid-column:span 12}.stats,.summary{grid-template-columns:repeat(2,minmax(0,1fr))}.history{grid-template-columns:1fr}.kv{grid-template-columns:1fr;gap:6px}}
    @media (max-width:640px){.stats,.summary{grid-template-columns:1fr}}
  </style>
</head>
<body>
  <main class="shell">
    <section class="panel hero">
      <p class="eyebrow">Home Assistant App</p>
      <div class="title">
        <div><h1 id="route-title">Garbage ETA Predictor</h1><p class="subtitle" id="route-subtitle">雙溪線 / 有謙家園</p></div>
        <div class="row"><span class="badge" id="service-badge">載入中</span><span class="badge" id="prediction-badge">等待資料</span></div>
      </div>
      <div class="grid">
        <section class="panel s7">
          <div class="muted">預測剩餘時間</div>
          <div class="hero-value" id="remaining-minutes">--</div>
          <div class="muted" id="arrival-at">尚未取得到站預測</div>
          <div class="stats">
            <div class="stat"><div class="stat-label">資料來源</div><div class="stat-value" id="prediction-source">待命中</div></div>
            <div class="stat"><div class="stat-label">信心等級</div><div class="stat-value" id="prediction-confidence">--</div></div>
            <div class="stat"><div class="stat-label">今日 sample 數</div><div class="stat-value" id="today-samples">0</div></div>
            <div class="stat"><div class="stat-label">今日 GPS sample</div><div class="stat-value" id="today-gps-samples">0</div></div>
          </div>
          <div class="message" id="status-message">正在讀取目前狀態...</div>
        </section>
        <section class="panel s5">
          <h2>今晚狀態</h2>
          <dl>
            <div class="kv"><dt>日期</dt><dd id="service-date">--</dd></div>
            <div class="kv"><dt>星期</dt><dd id="weekday">--</dd></div>
            <div class="kv"><dt>收集時窗</dt><dd id="collection-window">--</dd></div>
            <div class="kv"><dt>Run 狀態</dt><dd id="run-status">--</dd></div>
            <div class="kv"><dt>最近收集</dt><dd id="last-collected">--</dd></div>
            <div class="kv"><dt>已送提醒</dt><dd id="notified-offsets">--</dd></div>
            <div class="kv"><dt>輪詢間隔</dt><dd id="check-interval">--</dd></div>
            <div class="kv"><dt>共享路徑</dt><dd id="shared-data-path">--</dd></div>
            <div class="kv"><dt>最後更新</dt><dd id="updated-at">--</dd></div>
          </dl>
        </section>
        <section class="panel s6">
          <h2>車輛與站點</h2>
          <dl>
            <div class="kv"><dt>GPS 狀態</dt><dd id="gps-status">--</dd></div>
            <div class="kv"><dt>垃圾車座標</dt><dd id="truck-coords">--</dd></div>
            <div class="kv"><dt>目標站點</dt><dd id="target-coords">--</dd></div>
            <div class="kv"><dt>地圖</dt><dd><a id="map-link" href="#" target="_blank" rel="noreferrer">等待 GPS 後可產生地圖</a></dd></div>
          </dl>
        </section>
        <section class="panel s6">
          <h2>API 與即時除錯</h2>
          <dl>
            <div class="kv"><dt>EstimatedTime</dt><dd id="api-estimated">--</dd></div>
            <div class="kv"><dt>WaitingTime</dt><dd id="api-waiting">--</dd></div>
            <div class="kv"><dt>狀態 API</dt><dd><a id="status-link" href="./status" target="_blank" rel="noreferrer">開啟 /status JSON</a></dd></div>
          </dl>
          <pre id="raw-json">{"message":"loading"}</pre>
        </section>
        <section class="panel s12">
          <div class="toolbar">
            <div><h2>歷史資料</h2><div class="helper">可切換任一天的收集結果，直接看軌跡、摘要與 JSON / CSV 匯出。</div></div>
            <div class="row"><label for="history-date">選擇日期</label><select id="history-date"></select><a id="history-json-link" href="#" target="_blank" rel="noreferrer">JSON</a><a id="history-csv-link" href="#" target="_blank" rel="noreferrer">CSV</a></div>
          </div>
          <div class="history">
            <div>
              <div class="summary">
                <div class="card"><div class="stat-label">總 sample</div><div class="stat-value" id="history-sample-count">0</div></div>
                <div class="card"><div class="stat-label">有 GPS sample</div><div class="stat-value" id="history-gps-count">0</div></div>
                <div class="card"><div class="stat-label">第一筆</div><div class="stat-value" id="history-first">--</div></div>
                <div class="card"><div class="stat-label">最後一筆</div><div class="stat-value" id="history-last">--</div></div>
                <div class="card"><div class="stat-label">Run 狀態</div><div class="stat-value" id="history-run-status">--</div></div>
                <div class="card"><div class="stat-label">已送提醒</div><div class="stat-value" id="history-notified">--</div></div>
              </div>
              <dl>
                <div class="kv"><dt>到站時間</dt><dd id="history-arrival">--</dd></div>
                <div class="kv"><dt>共享資料</dt><dd id="history-shared-path">--</dd></div>
                <div class="kv"><dt>Collection Window</dt><dd id="history-window">--</dd></div>
              </dl>
              <div class="message" id="history-message">正在載入歷史資料...</div>
            </div>
            <div id="history-map"></div>
          </div>
          <div class="table-wrap">
            <table>
              <thead><tr><th>時間</th><th>GPS</th><th>座標</th><th>Progress</th><th>Segment</th><th>Lateral</th><th>API ETA</th><th>Waiting</th></tr></thead>
              <tbody id="history-table-body"><tr><td colspan="8" class="helper">尚未載入資料。</td></tr></tbody>
            </table>
          </div>
        </section>
        <section class="panel s12">
          <h2>HomePod Mini 測試播報</h2>
          <div class="row" style="display:grid;grid-template-columns:repeat(12,minmax(0,1fr));gap:14px">
            <div style="grid-column:span 12"><label for="broadcast-message">播報訊息</label><textarea id="broadcast-message" placeholder="例如：垃圾車測試廣播，請準備倒垃圾。"></textarea></div>
            <div style="grid-column:span 6"><label for="tts-entity">TTS 引擎</label><select id="tts-entity"></select></div>
            <div style="grid-column:span 6"><label for="tts-language">語言代碼（可留空）</label><input id="tts-language" type="text" placeholder="Gemini 請留空；其他可填 zh-TW 或 en"></div>
            <div style="grid-column:span 6"><label for="tts-voice">Gemini 聲線</label><select id="tts-voice"><option value="">自動（Gemini 預設用 achernar）</option><option value="achernar">achernar</option><option value="leda">leda</option><option value="kore">kore</option><option value="zephyr">zephyr</option></select></div>
            <div style="grid-column:span 12"><label>選擇要播報的 HomePod mini</label><div class="device-list" id="device-list"><div class="helper">正在讀取可用裝置...</div></div></div>
            <div style="grid-column:span 12"><div class="actions"><button id="broadcast-button" type="button" disabled>送出測試播報</button><span class="helper" id="broadcast-summary">請先輸入訊息並勾選至少一台 HomePod。</span></div><div class="message" id="broadcast-result">這裡會顯示送出結果。</div></div>
          </div>
        </section>
        <section class="panel s12">
          <h2>正式通知播報設定</h2>
          <div class="helper" style="margin-bottom:14px">垃圾車自動提醒會使用這組 TTS 引擎、聲線與 HomePod 目標。若 Gemini 作為引擎，語言欄位請留空。</div>
          <div class="row" style="display:grid;grid-template-columns:repeat(12,minmax(0,1fr));gap:14px">
            <div style="grid-column:span 6"><label for="auto-tts-entity">TTS 引擎</label><select id="auto-tts-entity"></select></div>
            <div style="grid-column:span 6"><label for="auto-tts-language">語言代碼（可留空）</label><input id="auto-tts-language" type="text" placeholder="Gemini 請留空；其他可填 zh-TW 或 en"></div>
            <div style="grid-column:span 6"><label for="auto-tts-voice">Gemini 聲線</label><select id="auto-tts-voice"><option value="">自動（Gemini 預設用 achernar）</option><option value="achernar">achernar</option><option value="leda">leda</option><option value="kore">kore</option><option value="zephyr">zephyr</option></select></div>
            <div style="grid-column:span 12"><label>自動提醒要播報的 HomePod mini</label><div class="device-list" id="auto-device-list"><div class="helper">正在讀取可用裝置...</div></div></div>
            <div style="grid-column:span 12"><div class="actions"><button id="auto-save-button" type="button" disabled>儲存正式播報設定</button><span class="helper" id="auto-summary">正在載入目前設定。</span></div><div class="message" id="auto-result">這裡會顯示正式播報設定的儲存結果。</div></div>
          </div>
        </section>
      </div>
    </section>
  </main>
  <script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js" crossorigin=""></script>
  <script>
    const statusURL=new URL("./status",window.location.href),historyDatesURL=new URL("./api/history/dates",window.location.href),historyTodayURL=new URL("./api/history/today",window.location.href),historyDayURL=new URL("./api/history/day",window.location.href),historyDayJSONURL=new URL("./api/history/day.json",window.location.href),historyDayCSVURL=new URL("./api/history/day.csv",window.location.href),broadcastOptionsURL=new URL("./api/broadcast/options",window.location.href),broadcastTestURL=new URL("./api/broadcast/test",window.location.href),broadcastAutoURL=new URL("./api/broadcast/auto",window.location.href);
    const els={routeTitle:document.getElementById("route-title"),routeSubtitle:document.getElementById("route-subtitle"),serviceBadge:document.getElementById("service-badge"),predictionBadge:document.getElementById("prediction-badge"),remainingMinutes:document.getElementById("remaining-minutes"),arrivalAt:document.getElementById("arrival-at"),predictionSource:document.getElementById("prediction-source"),predictionConfidence:document.getElementById("prediction-confidence"),todaySamples:document.getElementById("today-samples"),todayGPSSamples:document.getElementById("today-gps-samples"),statusMessage:document.getElementById("status-message"),serviceDate:document.getElementById("service-date"),weekday:document.getElementById("weekday"),collectionWindow:document.getElementById("collection-window"),runStatus:document.getElementById("run-status"),lastCollected:document.getElementById("last-collected"),notifiedOffsets:document.getElementById("notified-offsets"),checkInterval:document.getElementById("check-interval"),sharedDataPath:document.getElementById("shared-data-path"),updatedAt:document.getElementById("updated-at"),gpsStatus:document.getElementById("gps-status"),truckCoords:document.getElementById("truck-coords"),targetCoords:document.getElementById("target-coords"),mapLink:document.getElementById("map-link"),apiEstimated:document.getElementById("api-estimated"),apiWaiting:document.getElementById("api-waiting"),statusLink:document.getElementById("status-link"),rawJSON:document.getElementById("raw-json"),historyDate:document.getElementById("history-date"),historyJSONLink:document.getElementById("history-json-link"),historyCSVLink:document.getElementById("history-csv-link"),historySampleCount:document.getElementById("history-sample-count"),historyGPSCount:document.getElementById("history-gps-count"),historyFirst:document.getElementById("history-first"),historyLast:document.getElementById("history-last"),historyRunStatus:document.getElementById("history-run-status"),historyNotified:document.getElementById("history-notified"),historyArrival:document.getElementById("history-arrival"),historySharedPath:document.getElementById("history-shared-path"),historyWindow:document.getElementById("history-window"),historyMessage:document.getElementById("history-message"),historyTableBody:document.getElementById("history-table-body"),broadcastMessage:document.getElementById("broadcast-message"),ttsEntity:document.getElementById("tts-entity"),ttsLanguage:document.getElementById("tts-language"),ttsVoice:document.getElementById("tts-voice"),deviceList:document.getElementById("device-list"),broadcastButton:document.getElementById("broadcast-button"),broadcastSummary:document.getElementById("broadcast-summary"),broadcastResult:document.getElementById("broadcast-result"),autoTTSEntity:document.getElementById("auto-tts-entity"),autoTTSLanguage:document.getElementById("auto-tts-language"),autoTTSVoice:document.getElementById("auto-tts-voice"),autoDeviceList:document.getElementById("auto-device-list"),autoSaveButton:document.getElementById("auto-save-button"),autoSummary:document.getElementById("auto-summary"),autoResult:document.getElementById("auto-result")};
    let selectedHistoryDate="",historyMap,historyLayer,broadcastOptions={media_players:[],tts_entities:[],default_tts_entity:""},autoBroadcastSettings={target_entity_ids:[],tts_entity_id:"",language:"",voice:""};
    const fmt=(v)=>{if(!v)return"--";const d=new Date(v);return Number.isNaN(d.getTime())?v:new Intl.DateTimeFormat("zh-TW",{year:"numeric",month:"2-digit",day:"2-digit",hour:"2-digit",minute:"2-digit"}).format(d)},coords=(a,b)=>typeof a==="number"&&typeof b==="number"?a.toFixed(6)+", "+b.toFixed(6):"--",label=(m,k)=>m[k]||k||"--";
    function googleMapURL(a,b,c,d){const p=new URLSearchParams({api:"1",origin:a+","+b,destination:c+","+d,travelmode:"driving"});return "https://www.google.com/maps/dir/?"+p.toString()}
    function updateHistoryLinks(date){const j=new URL(historyDayJSONURL),c=new URL(historyDayCSVURL);j.searchParams.set("date",date);c.searchParams.set("date",date);els.historyJSONLink.href=j.toString();els.historyCSVLink.href=c.toString()}
    function ensureMap(){if(historyMap)return;historyMap=L.map("history-map");L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png",{maxZoom:19,attribution:"&copy; OpenStreetMap contributors"}).addTo(historyMap);historyLayer=L.layerGroup().addTo(historyMap);historyMap.setView([24.748448,121.02032],14)}
    function renderMap(day){ensureMap();historyLayer.clearLayers();const gps=(day.samples||[]).filter((s)=>s.gps_available&&typeof s.truck_lat==="number"&&typeof s.truck_lng==="number"),pts=gps.map((s)=>[s.truck_lat,s.truck_lng]),bounds=[];if(pts.length){L.polyline(pts,{color:"#95f2d2",weight:5,opacity:.9}).addTo(historyLayer);L.marker(pts[0]).addTo(historyLayer).bindPopup("起點");L.marker(pts[pts.length-1]).addTo(historyLayer).bindPopup("終點");bounds.push(...pts)}if(typeof day.target_lat==="number"&&typeof day.target_lng==="number"){const target=[day.target_lat,day.target_lng];L.circleMarker(target,{radius:8,color:"#ffd36b",fillColor:"#ffd36b",fillOpacity:.9}).addTo(historyLayer).bindPopup((day.point_name||"目標站點")+"（第 "+(day.point_seq||"--")+" 站）");bounds.push(target)}if(bounds.length)historyMap.fitBounds(bounds,{padding:[30,30]});else historyMap.setView([24.748448,121.02032],14)}
    function renderStatus(s){const p=s.prediction;els.routeTitle.textContent=s.route_name||"雙溪線";els.routeSubtitle.textContent=(s.point_name||"有謙家園")+" / "+(s.point_seq?("第 "+s.point_seq+" 站"):"目標站點");els.serviceBadge.textContent=s.active?"收集中":"待命中";els.serviceBadge.className="badge "+(s.active?"active":"inactive");if(p){els.predictionBadge.textContent=label({historical_model:"歷史模型",api_estimated_time:"API EstimatedTime",api_waiting_time:"API WaitingTime"},p.source);els.predictionBadge.className="badge active";els.remainingMinutes.textContent=p.remaining_minutes+" min";els.arrivalAt.textContent="預測到站 "+fmt(p.predicted_arrival_at);els.predictionSource.textContent=label({historical_model:"歷史模型",api_estimated_time:"API EstimatedTime",api_waiting_time:"API WaitingTime"},p.source);els.predictionConfidence.textContent=label({high:"高",medium:"中",low:"低"},p.confidence)}else{els.predictionBadge.textContent="等待資料";els.predictionBadge.className="badge inactive";els.remainingMinutes.textContent="--";els.arrivalAt.textContent="尚未取得到站預測";els.predictionSource.textContent="待命中";els.predictionConfidence.textContent="--"}els.todaySamples.textContent=String(s.today_sample_count||0);els.todayGPSSamples.textContent=String(s.today_gps_sample_count||0);els.statusMessage.textContent=s.message||"服務已啟動，等待下一次輪詢。";els.statusMessage.className="message";els.serviceDate.textContent=s.service_date||"--";els.weekday.textContent=s.weekday||"--";els.collectionWindow.textContent=s.collection_window||"--";els.runStatus.textContent=s.run_status||"--";els.lastCollected.textContent=fmt(s.last_collected_at);els.notifiedOffsets.textContent=(s.notified_offsets||[]).length?s.notified_offsets.map((o)=>o+" 分鐘").join(" / "):"尚未送出";els.checkInterval.textContent=s.check_interval||"--";els.sharedDataPath.textContent=s.shared_data_path||"--";els.updatedAt.textContent=fmt(s.updated_at);els.gpsStatus.textContent=s.gps_available?"可用":"目前不可用";els.truckCoords.textContent=s.gps_available?coords(s.truck_lat,s.truck_lng):"--";els.targetCoords.textContent=coords(s.target_lat,s.target_lng);els.apiEstimated.textContent=s.api_estimated_text||"--";els.apiWaiting.textContent=s.api_waiting_time===null||s.api_waiting_time===undefined?"--":String(s.api_waiting_time);els.rawJSON.textContent=JSON.stringify(s,null,2);els.statusLink.href=statusURL.toString();if(s.gps_available&&typeof s.target_lat==="number"&&typeof s.target_lng==="number"){els.mapLink.href=googleMapURL(s.truck_lat,s.truck_lng,s.target_lat,s.target_lng);els.mapLink.textContent="開啟垃圾車與站點路線"}else{els.mapLink.href="#";els.mapLink.textContent="等待 GPS 後可產生地圖"}}
    function renderHistory(day){updateHistoryLinks(day.service_date);els.historySampleCount.textContent=String(day.sample_count||0);els.historyGPSCount.textContent=String(day.gps_sample_count||0);els.historyFirst.textContent=fmt(day.first_collected_at);els.historyLast.textContent=fmt(day.last_collected_at);els.historyRunStatus.textContent=day.run_status||"尚未開始";els.historyNotified.textContent=(day.notified_offsets||[]).length?day.notified_offsets.map((o)=>o+" 分鐘").join(" / "):"尚未送出";els.historyArrival.textContent=fmt(day.arrival_at);els.historySharedPath.textContent=day.shared_data_path||"--";els.historyWindow.textContent=day.collection_window||"--";els.historyMessage.textContent=(day.samples||[]).length?"已載入 "+day.service_date+" 的完整收集紀錄。":"這一天還沒有 sample。";els.historyMessage.className="message";els.historyTableBody.innerHTML=(day.samples||[]).length?(day.samples||[]).map((s)=>"<tr><td>"+fmt(s.collected_at)+"</td><td>"+(s.gps_available?"有":"無")+"</td><td>"+(s.gps_available?coords(s.truck_lat,s.truck_lng):"--")+"</td><td>"+(s.progress_meters===null||s.progress_meters===undefined?"--":s.progress_meters.toFixed(1)+" m")+"</td><td>"+(s.segment_index===null||s.segment_index===undefined?"--":s.segment_index)+"</td><td>"+(s.lateral_offset_meters===null||s.lateral_offset_meters===undefined?"--":s.lateral_offset_meters.toFixed(1)+" m")+"</td><td>"+(s.api_estimated_minutes===null||s.api_estimated_minutes===undefined?(s.api_estimated_text||"--"):s.api_estimated_minutes+" 分鐘")+"</td><td>"+(s.api_waiting_time===null||s.api_waiting_time===undefined?"--":s.api_waiting_time)+"</td></tr>").join(""):"<tr><td colspan='8' class='helper'>這一天沒有資料。</td></tr>";renderMap(day)}
    async function refreshStatus(){try{const r=await fetch(statusURL,{cache:"no-store"});if(!r.ok)throw new Error("HTTP "+r.status);renderStatus(await r.json())}catch(e){els.statusMessage.textContent="狀態讀取失敗："+e.message;els.statusMessage.className="message error"}}
    async function loadHistoryDay(date){selectedHistoryDate=date;updateHistoryLinks(date);const u=new URL(historyDayURL);u.searchParams.set("date",date);try{const r=await fetch(u,{cache:"no-store"});if(!r.ok)throw new Error("HTTP "+r.status);renderHistory(await r.json())}catch(e){els.historyMessage.textContent="歷史資料讀取失敗："+e.message;els.historyMessage.className="message error";els.historyTableBody.innerHTML="<tr><td colspan='8' class='helper'>讀取失敗。</td></tr>"}}
    async function loadHistoryDates(){const [dr,tr]=await Promise.all([fetch(historyDatesURL,{cache:"no-store"}),fetch(historyTodayURL,{cache:"no-store"})]);if(!dr.ok)throw new Error("dates HTTP "+dr.status);if(!tr.ok)throw new Error("today HTTP "+tr.status);const dp=await dr.json(),today=await tr.json(),dates=dp.dates||[],preferred=dates.includes(today.service_date)?today.service_date:(dates[0]||today.service_date);els.historyDate.innerHTML="";Array.from(new Set([preferred,...dates])).filter(Boolean).forEach((d)=>{const o=document.createElement("option");o.value=d;o.textContent=d;o.selected=d===preferred;els.historyDate.appendChild(o)});selectedHistoryDate=preferred;if(today.service_date===preferred)renderHistory(today);else await loadHistoryDay(preferred)}
    function selectedTargets(groupName){return Array.from(document.querySelectorAll("input[name='"+groupName+"']:checked")).map((n)=>n.value)}
    function updateBroadcastButtonState(){const hasMessage=els.broadcastMessage.value.trim().length>0,targets=selectedTargets("broadcast-target"),isGemini=els.ttsEntity.value==="tts.google_ai_tts"||els.ttsEntity.value==="tts.google_generative_ai_tts";els.broadcastButton.disabled=!(hasMessage&&targets.length>0&&els.ttsEntity.value);if(!hasMessage)els.broadcastSummary.textContent="請先輸入測試播報內容。";else if(targets.length===0)els.broadcastSummary.textContent="請至少勾選一台要播報的 HomePod。";else if(!els.ttsEntity.value)els.broadcastSummary.textContent="目前找不到可用的 TTS 引擎。";else if(isGemini)els.broadcastSummary.textContent="Gemini 會自動判斷中文，語言欄位請留空；目前聲線為 "+(els.ttsVoice.value||"achernar")+"。";else els.broadcastSummary.textContent="將送到 "+targets.length+" 台裝置。"}
    function updateAutoBroadcastButtonState(){const targets=selectedTargets("auto-broadcast-target"),isGemini=els.autoTTSEntity.value==="tts.google_ai_tts"||els.autoTTSEntity.value==="tts.google_generative_ai_tts";els.autoSaveButton.disabled=!(targets.length>0&&els.autoTTSEntity.value);if(targets.length===0)els.autoSummary.textContent="請至少勾選一台正式提醒要播報的 HomePod。";else if(!els.autoTTSEntity.value)els.autoSummary.textContent="目前找不到可用的 TTS 引擎。";else if(isGemini)els.autoSummary.textContent="正式通知將使用 Gemini，自動判斷中文，語言欄位請留空；聲線為 "+(els.autoTTSVoice.value||"achernar")+"。";else els.autoSummary.textContent="正式通知將送到 "+targets.length+" 台裝置。"}
    function renderDeviceCheckboxes(container,groupName,selected){container.innerHTML="";if((broadcastOptions.media_players||[]).length===0){container.innerHTML="<div class='helper'>目前找不到可用的 HomePod 或 media_player。</div>";return}const selectedSet=new Set(selected||[]);broadcastOptions.media_players.forEach((device,index)=>{const label=document.createElement("label");label.className="device";const input=document.createElement("input");input.type="checkbox";input.name=groupName;input.value=device.entity_id;input.checked=selectedSet.size?selectedSet.has(device.entity_id):index===0;const meta=document.createElement("div");meta.innerHTML="<strong>"+device.friendly_name+"</strong><div class='helper'>"+device.entity_id+" / "+device.state+"</div>";label.appendChild(input);label.appendChild(meta);container.appendChild(label)})}
    function renderTTSEntitySelect(selectEl,selectedEntityID){selectEl.innerHTML="";if((broadcastOptions.tts_entities||[]).length===0){const option=document.createElement("option");option.value="";option.textContent="找不到可用 TTS";selectEl.appendChild(option);return}broadcastOptions.tts_entities.forEach((entity)=>{const option=document.createElement("option");option.value=entity.entity_id;option.textContent=entity.friendly_name+" ("+entity.entity_id+")";option.selected=entity.entity_id===(selectedEntityID||broadcastOptions.default_tts_entity);selectEl.appendChild(option)})}
    function renderBroadcastOptions(o){broadcastOptions=o||{media_players:[],tts_entities:[],default_tts_entity:""};renderTTSEntitySelect(els.ttsEntity,broadcastOptions.default_tts_entity);renderDeviceCheckboxes(els.deviceList,"broadcast-target",[]);Array.from(document.querySelectorAll("input[name='broadcast-target']")).forEach((input)=>input.addEventListener("change",updateBroadcastButtonState));updateBroadcastButtonState()}
    function applyAutoBroadcastSettings(settings){autoBroadcastSettings=settings||{target_entity_ids:[],tts_entity_id:broadcastOptions.default_tts_entity||"",language:"",voice:""};renderTTSEntitySelect(els.autoTTSEntity,autoBroadcastSettings.tts_entity_id);els.autoTTSLanguage.value=autoBroadcastSettings.language||"";els.autoTTSVoice.value=autoBroadcastSettings.voice||"";renderDeviceCheckboxes(els.autoDeviceList,"auto-broadcast-target",autoBroadcastSettings.target_entity_ids||[]);Array.from(document.querySelectorAll("input[name='auto-broadcast-target']")).forEach((input)=>input.addEventListener("change",updateAutoBroadcastButtonState));updateAutoBroadcastButtonState()}
    async function loadBroadcastOptions(){try{const r=await fetch(broadcastOptionsURL,{cache:"no-store"});if(!r.ok)throw new Error("HTTP "+r.status);renderBroadcastOptions(await r.json());els.broadcastResult.textContent="已載入可用的播報裝置與 TTS 引擎。";els.broadcastResult.className="message"}catch(e){els.deviceList.innerHTML="<div class='helper'>裝置清單讀取失敗："+e.message+"</div>";els.autoDeviceList.innerHTML="<div class='helper'>裝置清單讀取失敗："+e.message+"</div>";els.broadcastResult.textContent="無法讀取播報設定："+e.message;els.broadcastResult.className="message error";els.autoResult.textContent="無法讀取正式播報設定："+e.message;els.autoResult.className="message error";updateBroadcastButtonState();updateAutoBroadcastButtonState()}}
    async function loadAutoBroadcastSettings(){try{const r=await fetch(broadcastAutoURL,{cache:"no-store"});if(!r.ok)throw new Error("HTTP "+r.status);applyAutoBroadcastSettings(await r.json());els.autoResult.textContent="已載入正式通知播報設定。";els.autoResult.className="message success"}catch(e){els.autoResult.textContent="無法讀取正式播報設定："+e.message;els.autoResult.className="message error";updateAutoBroadcastButtonState()}}
    async function submitBroadcast(){const payload={message:els.broadcastMessage.value.trim(),target_entity_ids:selectedTargets("broadcast-target"),tts_entity_id:els.ttsEntity.value,language:els.ttsLanguage.value.trim(),voice:els.ttsVoice.value};els.broadcastButton.disabled=true;els.broadcastResult.textContent="正在送出測試播報...";els.broadcastResult.className="message";try{const r=await fetch(broadcastTestURL,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)}),text=await r.text();if(!r.ok)throw new Error(text||("HTTP "+r.status));els.broadcastResult.textContent="已送出測試播報到："+payload.target_entity_ids.join(" / ");els.broadcastResult.className="message success"}catch(e){els.broadcastResult.textContent="播報送出失敗："+e.message;els.broadcastResult.className="message error"}finally{updateBroadcastButtonState()}}
    async function saveAutoBroadcastSettings(){const payload={target_entity_ids:selectedTargets("auto-broadcast-target"),tts_entity_id:els.autoTTSEntity.value,language:els.autoTTSLanguage.value.trim(),voice:els.autoTTSVoice.value};els.autoSaveButton.disabled=true;els.autoResult.textContent="正在儲存正式播報設定...";els.autoResult.className="message";try{const r=await fetch(broadcastAutoURL,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)}),text=await r.text();if(!r.ok)throw new Error(text||("HTTP "+r.status));applyAutoBroadcastSettings(JSON.parse(text));els.autoResult.textContent="正式通知播報設定已更新。";els.autoResult.className="message success"}catch(e){els.autoResult.textContent="儲存正式播報設定失敗："+e.message;els.autoResult.className="message error"}finally{updateAutoBroadcastButtonState()}}
    els.broadcastMessage.addEventListener("input",updateBroadcastButtonState);els.ttsEntity.addEventListener("change",updateBroadcastButtonState);els.ttsVoice.addEventListener("change",updateBroadcastButtonState);els.broadcastButton.addEventListener("click",submitBroadcast);els.autoTTSEntity.addEventListener("change",updateAutoBroadcastButtonState);els.autoTTSVoice.addEventListener("change",updateAutoBroadcastButtonState);els.autoSaveButton.addEventListener("click",saveAutoBroadcastSettings);els.historyDate.addEventListener("change",(e)=>loadHistoryDay(e.target.value));
    refreshStatus();loadBroadcastOptions().then(loadAutoBroadcastSettings);loadHistoryDates().catch((e)=>{els.historyMessage.textContent="歷史資料初始化失敗："+e.message;els.historyMessage.className="message error"});window.setInterval(async()=>{await refreshStatus();if(selectedHistoryDate)await loadHistoryDay(selectedHistoryDate)},30000);
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
