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
    :root{
      color-scheme:dark;
      --bg:#061015;
      --panel:#0e1b22;
      --panel2:#13252f;
      --panel3:#182f3b;
      --line:#243a46;
      --text:#f3faf8;
      --muted:#96ada8;
      --accent:#5ce0b8;
      --accent-dim:rgba(92,224,184,.14);
      --warn:#ffb347;
      --warn-dim:rgba(255,179,71,.16);
      --danger:#ff7b7b;
      --radius:12px;
      --radius-sm:8px;
      font-family:"IBM Plex Sans","Noto Sans TC","Microsoft JhengHei","Segoe UI",sans-serif;
    }
    *{box-sizing:border-box}
    body{margin:0;background:linear-gradient(180deg,#061015 0%,#0a171e 100%);color:var(--text);font-family:inherit;line-height:1.45}
    a{color:var(--accent);text-decoration:none}
    a:hover{text-decoration:underline}
    .shell{max-width:1440px;margin:0 auto;padding:16px 20px 32px}
    .panel{background:var(--panel);border:1px solid var(--line);border-radius:var(--radius);padding:18px}
    .subpanel{background:var(--panel2);border:1px solid var(--line);border-radius:var(--radius-sm);padding:14px}

    /* Top Persistent Header */
    .top-bar{display:flex;justify-content:space-between;align-items:center;gap:16px;flex-wrap:wrap;padding:16px 20px;background:linear-gradient(135deg,#0f2029 0%,#122834 100%);border:1px solid var(--line);border-radius:var(--radius)}
    .brand{display:flex;flex-direction:column;gap:4px}
    .eyebrow{font-size:.73rem;letter-spacing:.14em;text-transform:uppercase;color:var(--accent);margin:0}
    h1,h2,h3{margin:0}
    h1{font-size:clamp(1.35rem,2.2vw,1.8rem);font-weight:750}
    h2{font-size:1.05rem;font-weight:700;margin-bottom:10px}
    h3{font-size:.92rem;font-weight:650;color:var(--muted);margin-bottom:8px}
    .subtitle,.muted,.helper,label,th,dt{color:var(--muted)}
    .helper{font-size:.85rem}

    .top-metrics{display:flex;align-items:center;gap:12px;flex-wrap:wrap}
    .hero-pill{display:flex;align-items:baseline;gap:10px;padding:10px 16px;border-radius:10px;background:var(--panel3);border:1px solid var(--line)}
    .hero-pill .big{font-size:1.65rem;font-weight:800;color:var(--accent);line-height:1}
    .hero-pill .sub{font-size:.84rem;color:var(--muted)}
    .chip-group{display:flex;gap:8px;flex-wrap:wrap;align-items:center}
    .badge{display:inline-flex;align-items:center;gap:6px;padding:6px 12px;border-radius:999px;background:#162933;border:1px solid var(--line);font-size:.84rem;font-weight:600}
    .badge.active{color:var(--accent);border-color:rgba(92,224,184,.45);background:var(--accent-dim)}
    .badge.loop{color:var(--warn);border-color:rgba(255,179,71,.5);background:var(--warn-dim)}
    .badge.inactive{color:var(--muted)}
    .badge.error{color:var(--danger);border-color:rgba(255,123,123,.4)}

    /* Main Tabs */
    .tabs{display:flex;gap:10px;margin-top:14px;border-bottom:1px solid var(--line);padding-bottom:10px;flex-wrap:wrap}
    .tab-btn{appearance:none;border:1px solid var(--line);background:var(--panel);color:var(--muted);padding:11px 18px;border-radius:10px;font:inherit;font-weight:700;font-size:.95rem;cursor:pointer;transition:all .15s ease;display:inline-flex;align-items:center;gap:8px}
    .tab-btn:hover{color:var(--text);border-color:#385868;background:var(--panel2)}
    .tab-btn.active{background:linear-gradient(135deg,var(--accent),#43b89c);color:#061216;border-color:transparent;box-shadow:0 4px 14px rgba(92,224,184,.22)}
    .tab-pane{margin-top:14px}
    .hidden{display:none!important}

    /* Layout Grids */
    .grid{display:grid;grid-template-columns:repeat(12,minmax(0,1fr));gap:14px}
    .s12{grid-column:span 12}.s8{grid-column:span 8}.s7{grid-column:span 7}.s6{grid-column:span 6}.s5{grid-column:span 5}.s4{grid-column:span 4}
    .stats-4{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}
    .stats-2{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px}
    .stat{padding:12px 14px;border-radius:var(--radius-sm);background:var(--panel2);border:1px solid var(--line)}
    .stat-label{font-size:.76rem;color:var(--muted);margin-bottom:4px}
    .stat-value{font-size:1.12rem;font-weight:700}
    .stat-sub{font-size:.78rem;color:var(--muted);margin-top:2px}

    /* Timeline Stepper */
    .timeline{display:flex;flex-direction:column;gap:8px;margin:10px 0 14px}
    .step{display:flex;align-items:center;gap:11px;padding:9px 12px;border-radius:var(--radius-sm);background:var(--panel2);border:1px solid var(--line);font-size:.88rem}
    .step-dot{width:22px;height:22px;border-radius:50%;display:inline-flex;align-items:center;justify-content:center;font-size:.75rem;font-weight:800;background:#1a2e39;color:var(--muted);border:1px solid var(--line);flex-shrink:0}
    .step.done{border-color:rgba(92,224,184,.35);background:rgba(92,224,184,.07)}
    .step.done .step-dot{background:var(--accent);color:#061216;border-color:transparent}
    .step.current{border-color:var(--warn);background:var(--warn-dim)}
    .step.current .step-dot{background:var(--warn);color:#061216;border-color:transparent}
    .step-body{display:flex;justify-content:space-between;align-items:center;width:100%;gap:8px}
    .step-meta{font-size:.78rem;color:var(--muted)}

    /* Maps */
    .map-header{display:flex;justify-content:space-between;align-items:center;gap:10px;flex-wrap:wrap;margin-bottom:10px}
    .map-box{height:490px;border-radius:var(--radius-sm);overflow:hidden;border:1px solid var(--line);background:#091318}
    .legend{display:flex;gap:14px;flex-wrap:wrap;font-size:.82rem;color:var(--muted)}
    .legend span{display:inline-flex;align-items:center;gap:6px}
    .swatch{width:14px;height:4px;border-radius:2px;display:inline-block}
    .swatch.approach{background:#62d6b0}
    .swatch.loop{background:#ffb347;height:5px}
    .swatch.c80{width:10px;height:10px;border-radius:50%;border:2px dashed #ffb347;background:transparent}
    .swatch.c500{width:10px;height:10px;border-radius:50%;border:2px dashed #5bc0be;background:transparent}

    /* Controls & Buttons */
    .toolbar,.row,.actions{display:flex;gap:10px;flex-wrap:wrap;align-items:center}
    .toolbar{justify-content:space-between;margin-bottom:12px}
    select,input,textarea,button{font:inherit;border-radius:var(--radius-sm);border:1px solid var(--line);background:#09141a;color:var(--text);padding:10px 12px}
    select,input{min-height:42px;width:100%}
    textarea{width:100%;min-height:84px;resize:vertical}
    button{cursor:pointer;font-weight:700;transition:all .15s ease}
    .btn-primary{border:0;background:linear-gradient(135deg,var(--accent),#3db895);color:#061216;padding:11px 18px}
    .btn-deploy{border:0;background:linear-gradient(135deg,var(--warn),#f2994a);color:#091216;padding:11px 18px}
    .btn-ghost{background:var(--panel2);color:var(--text);border:1px solid var(--line);padding:7px 12px;font-size:.84rem;font-weight:600}
    .btn-ghost:hover{border-color:var(--accent);color:var(--accent)}
    .btn-ghost.active{background:var(--accent-dim);color:var(--accent);border-color:var(--accent)}
    button:disabled{opacity:.5;cursor:not-allowed}

    /* Unified Broadcast Studio */
    .deployed-bar{display:flex;justify-content:space-between;align-items:center;gap:14px;flex-wrap:wrap;padding:14px 16px;border-radius:var(--radius-sm);background:linear-gradient(135deg,rgba(92,224,184,.1) 0%,rgba(19,37,47,.95) 100%);border:1px solid rgba(92,224,184,.35);margin-bottom:16px}
    .deployed-grid{display:flex;gap:18px;flex-wrap:wrap;font-size:.88rem}
    .deployed-item strong{color:var(--accent);margin-left:4px}
    .tier-pills{display:flex;gap:8px;flex-wrap:wrap;margin-top:6px}
    .tier-pill{background:var(--panel2);color:var(--muted);border:1px solid var(--line);padding:9px 14px;border-radius:var(--radius-sm);font-size:.88rem;font-weight:650;cursor:pointer}
    .tier-pill.active{background:var(--accent-dim);color:var(--accent);border-color:var(--accent)}
    .device-list{display:grid;grid-template-columns:repeat(auto-fit,minmax(230px,1fr));gap:10px;margin-top:6px}
    .device{display:flex;gap:12px;align-items:flex-start;padding:12px;border-radius:var(--radius-sm);background:var(--panel2);border:1px solid var(--line);cursor:pointer}
    .device input{width:18px;height:18px;min-height:auto;margin-top:2px;accent-color:var(--accent)}
    .message{margin-top:10px;padding:11px 14px;border-radius:var(--radius-sm);background:#0c1820;color:var(--muted);border:1px solid var(--line);font-size:.9rem}
    .message.error{color:var(--danger);border-color:rgba(255,123,123,.4)}
    .message.success{color:#a4f4bc;border-color:rgba(92,224,184,.4)}
    .message.warn{color:var(--warn);border-color:rgba(255,179,71,.4)}

    /* Tables & Details */
    .table-wrap{overflow:auto;max-height:380px;border-radius:var(--radius-sm);border:1px solid var(--line);background:#09141a;margin-top:10px}
    table{width:100%;border-collapse:collapse;font-size:.86rem}
    th,td{padding:8px 10px;border-bottom:1px solid var(--line);text-align:left;vertical-align:middle;white-space:nowrap}
    thead th{position:sticky;top:0;background:#152731;z-index:1}
    tr.loop-row{background:rgba(255,179,71,.08)}
    details{margin-top:14px;background:var(--panel);border:1px solid var(--line);border-radius:var(--radius);padding:12px 16px}
    summary{cursor:pointer;font-weight:650;color:var(--muted);font-size:.9rem}
    pre{margin:10px 0 0;padding:12px;border-radius:var(--radius-sm);background:#081217;overflow:auto;max-height:240px;color:#d8f2e9;font-size:.8rem;border:1px solid var(--line)}
    .kv-compact{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:10px;margin-top:12px;font-size:.86rem}

    @media (max-width:980px){
      .s8,.s7,.s6,.s5,.s4{grid-column:span 12}
      .stats-4{grid-template-columns:repeat(2,minmax(0,1fr))}
      .map-box{height:380px}
    }
    @media (max-width:600px){
      .stats-4,.stats-2{grid-template-columns:1fr}
    }
  </style>
</head>
<body>
  <main class="shell">
    <!-- Top Persistent Status Bar -->
    <header class="top-bar">
      <div class="brand">
        <p class="eyebrow">Home Assistant · Garbage ETA Predictor</p>
        <div class="row">
          <h1 id="route-title">雙溪線</h1>
          <span class="badge" id="route-subtitle">有謙家園（第 27 站）</span>
          <span class="badge" id="service-badge">載入中</span>
          <span class="badge" id="prediction-badge">等待資料</span>
        </div>
      </div>
      <div class="top-metrics">
        <div class="hero-pill">
          <div>
            <div class="stat-label">預測到站倒數</div>
            <div class="big" id="remaining-minutes">--</div>
          </div>
          <div>
            <div class="sub" id="arrival-at">尚未取得到站預測</div>
            <div class="sub" id="top-distance">距有謙家園：--</div>
          </div>
        </div>
        <div class="chip-group">
          <div class="stat"><div class="stat-label">預測來源 / 信心</div><div class="stat-value"><span id="prediction-source">待命中</span> · <span id="prediction-confidence">--</span></div></div>
          <div class="stat"><div class="stat-label">今日軌跡點 (GPS)</div><div class="stat-value"><span id="today-gps-samples">0</span> / <span id="today-samples">0</span></div></div>
          <div class="stat"><div class="stat-label">今晚已送提醒</div><div class="stat-value" id="notified-offsets">尚未送出</div></div>
        </div>
      </div>
    </header>

    <!-- Navigation Tabs -->
    <nav class="tabs" aria-label="主要功能分頁">
      <button type="button" class="tab-btn active" data-tab="tab-live">📍 即時地圖與今日軌跡</button>
      <button type="button" class="tab-btn" data-tab="tab-broadcast">🔊 HomePod Mini 測試播報＆一鍵部署</button>
      <button type="button" class="tab-btn" data-tab="tab-history">📊 歷史資料與 ETA 準確度</button>
    </nav>

    <!-- TAB 1: Live Status & Map -->
    <section id="tab-live" class="tab-pane">
      <div class="grid">
        <div class="panel s4">
          <h2>今晚清運進度時間軸</h2>
          <div class="helper" id="status-message">正在讀取目前狀態...</div>
          <div class="timeline" id="timeline-steps">
            <div class="step" id="step-window"><span class="step-dot">1</span><div class="step-body"><span>收集時窗啟動</span><span class="step-meta" id="collection-window">19:00-21:30</span></div></div>
            <div class="step" id="step-gps"><span class="step-dot">2</span><div class="step-body"><span>垃圾車發車・GPS 追蹤</span><span class="step-meta" id="step-gps-meta">待命</span></div></div>
            <div class="step" id="step-alert10"><span class="step-dot">3</span><div class="step-body"><span>10 分鐘前提醒（歷史模型）</span><span class="step-meta" id="step-alert10-meta">未觸發</span></div></div>
            <div class="step" id="step-alert3"><span class="step-dot">4</span><div class="step-body"><span>3 分鐘前提醒</span><span class="step-meta" id="step-alert3-meta">未觸發</span></div></div>
            <div class="step" id="step-loop"><span class="step-dot">5</span><div class="step-body"><span>進入有謙家園・記錄繞行路線</span><span class="step-meta" id="step-loop-meta">80m 到站圈</span></div></div>
            <div class="step" id="step-done"><span class="step-dot">6</span><div class="step-body"><span>離開社區 (&gt;500m) / 今日完成</span><span class="step-meta" id="step-done-meta">未完成</span></div></div>
          </div>

          <h2>即時車輛與官方 API 對照</h2>
          <div class="stats-2">
            <div class="stat"><div class="stat-label">目前距有謙家園</div><div class="stat-value" id="live-distance">--</div><div class="stat-sub" id="gps-status">GPS 待命</div></div>
            <div class="stat"><div class="stat-label">官方 API 預估</div><div class="stat-value" id="api-estimated">--</div><div class="stat-sub">Waiting: <span id="api-waiting">--</span></div></div>
            <div class="stat"><div class="stat-label">垃圾車即時座標</div><div class="stat-value" style="font-size:.9rem" id="truck-coords">--</div><div class="stat-sub"><a id="map-link" href="#" target="_blank" rel="noreferrer">Google Maps 導航</a></div></div>
            <div class="stat"><div class="stat-label">最近收集 / 更新</div><div class="stat-value" style="font-size:.9rem" id="last-collected">--</div><div class="stat-sub">更新: <span id="updated-at">--</span></div></div>
          </div>
        </div>

        <div class="panel s8">
          <div class="map-header">
            <div>
              <h2>即時路線與「有謙家園」社區範圍地圖</h2>
              <div class="helper" id="live-map-banner">正在載入今日或最近一次清運軌跡...</div>
            </div>
            <div class="row">
              <button type="button" class="btn-ghost" id="btn-live-zoom-target">🎯 聚焦有謙家園 (500m)</button>
              <button type="button" class="btn-ghost" id="btn-live-zoom-full">🗺️ 顯示完整路線</button>
            </div>
          </div>
          <div class="legend" style="margin-bottom:10px">
            <span><i class="swatch approach"></i> 到站前主路線</span>
            <span><i class="swatch loop"></i> 進入有謙家園 / 社區繞行軌跡</span>
            <span><i class="swatch c80"></i> 80m 到站判定圈</span>
            <span><i class="swatch c500"></i> 500m 社區繞行圈</span>
          </div>
          <div id="live-map" class="map-box"></div>
        </div>
      </div>

      <details>
        <summary>🔧 展開進階系統資訊與原始 /status JSON 除錯資料</summary>
        <div class="kv-compact">
          <div><span class="muted">服務日期：</span><strong id="service-date">--</strong> (<span id="weekday">--</span>)</div>
          <div><span class="muted">Run 狀態：</span><strong id="run-status">--</strong></div>
          <div><span class="muted">清運日設定：</span><strong id="target-days">--</strong></div>
          <div><span class="muted">輪詢間隔：</span><strong id="check-interval">--</strong></div>
          <div><span class="muted">有謙家園座標：</span><strong id="target-coords">--</strong></div>
          <div><span class="muted">共享資料夾：</span><strong id="shared-data-path">--</strong></div>
          <div><span class="muted">狀態端點：</span><a id="status-link" href="./status" target="_blank" rel="noreferrer">開啟 /status JSON</a></div>
        </div>
        <pre id="raw-json">{"message":"loading"}</pre>
      </details>
    </section>

    <!-- TAB 2: Unified HomePod Broadcast Studio -->
    <section id="tab-broadcast" class="tab-pane hidden">
      <div class="panel">
        <div class="toolbar">
          <div>
            <h2>HomePod Mini 測試播報與正式通知設定（合一工作台）</h2>
            <div class="helper">在同一個介面直接挑選聲線、調整語速並按「立即試聽」，確認滿意後點「一鍵部署為正式通知設定」即可套用。</div>
          </div>
          <span class="badge active" id="diff-badge">✓ 與目前正式設定一致</span>
        </div>

        <!-- Currently Deployed Settings Banner -->
        <div class="deployed-bar">
          <div>
            <div class="eyebrow" style="margin-bottom:4px">目前已部署的正式通知設定（垃圾車自動提醒使用中）</div>
            <div class="deployed-grid">
              <div class="deployed-item">TTS 引擎：<strong id="dep-engine">載入中...</strong></div>
              <div class="deployed-item">Voice / 聲線：<strong id="dep-voice">--</strong></div>
              <div class="deployed-item">語音參數：<strong id="dep-params">--</strong></div>
              <div class="deployed-item">播報 HomePod：<strong id="dep-targets">--</strong></div>
            </div>
          </div>
          <button type="button" class="btn-ghost hidden" id="reset-to-deployed-btn">↩️ 放棄修改・還原為正式設定</button>
        </div>

        <div class="grid">
          <!-- Left: Engine, Tier & Voice, Prosody -->
          <div class="subpanel s7">
            <h3>步驟 1：選擇 TTS 引擎與聲線</h3>
            <div class="grid">
              <div class="s8">
                <label for="tts-entity">TTS 引擎（預設 Google Cloud TTS，失敗時自動備援 Google Translate）</label>
                <select id="tts-entity"></select>
              </div>
              <div class="s4">
                <label for="tts-language">語言代碼</label>
                <input id="tts-language" type="text" placeholder="cmn-TW 或 zh-tw">
              </div>
            </div>

            <div id="voice-tier-section" style="margin-top:14px">
              <label>快速篩選 Google Cloud 聲線系列</label>
              <div class="tier-pills" id="tier-pills">
                <button type="button" class="tier-pill active" data-tier="WaveNet">🇹🇼 WaveNet（台灣口音・可調語速/音高）</button>
                <button type="button" class="tier-pill" data-tier="Neural2">✨ Neural2（自然女聲）</button>
                <button type="button" class="tier-pill" data-tier="Chirp 3: HD">🎙️ Chirp 3: HD（高擬真對話・30 款聲線）</button>
                <button type="button" class="tier-pill" data-tier="ALL">全部顯示</button>
              </div>
            </div>

            <div style="margin-top:12px">
              <label for="tts-voice">Voice / 聲線模型</label>
              <select id="tts-voice"></select>
            </div>

            <!-- Google Cloud Prosody Controls -->
            <div id="tts-cloud-wrap" style="margin-top:14px">
              <h3>步驟 2：語音微調參數</h3>
              <div id="chirp-hint" class="message warn hidden" style="margin-bottom:10px">
                💡 <strong>Chirp 3: HD</strong> 為新一代生成式語音模型，由標點符號（逗號、句號）自然控制停頓與語氣，因此<strong>不支援手動調整語速、音高與 SSML</strong>；下方音量增益與喇叭音效仍可使用。
              </div>
              <div class="grid" id="prosody-controls">
                <div class="s4">
                  <label for="tts-input-mode">輸入模式</label>
                  <select id="tts-input-mode"><option value="text">Text（一般文字）</option><option value="ssml">SSML 標記</option></select>
                </div>
                <div class="s4">
                  <label for="tts-speaking-rate">語速 (0.25 ~ 4.0)</label>
                  <input id="tts-speaking-rate" type="number" step="0.05" min="0.25" max="4" placeholder="1.0（預設）">
                </div>
                <div class="s4">
                  <label for="tts-pitch">音高 (-20 ~ 20)</label>
                  <input id="tts-pitch" type="number" step="0.5" min="-20" max="20" placeholder="0（預設）">
                </div>
              </div>
              <div class="grid" style="margin-top:10px">
                <div class="s6">
                  <label for="tts-volume-gain-db">音量增益 dB (-96 ~ +16)</label>
                  <input id="tts-volume-gain-db" type="number" step="0.5" min="-96" max="16" placeholder="0（預設）">
                </div>
                <div class="s6">
                  <label for="tts-effects-profile">喇叭音效最佳化 (Effects Profile)</label>
                  <input id="tts-effects-profile" type="text" placeholder="例如：small-bluetooth-speaker-class-device">
                </div>
              </div>
            </div>
          </div>

          <!-- Right: HomePod Targets, Test Message & One-Click Deploy -->
          <div class="subpanel s5">
            <h3>步驟 3：選擇要播報的 HomePod mini（試聽與正式通知共用）</h3>
            <div class="device-list" id="device-list">
              <div class="helper">正在讀取可用 HomePod 裝置...</div>
            </div>

            <div style="margin-top:16px">
              <div class="toolbar" style="margin-bottom:6px">
                <label for="broadcast-message">步驟 4：試聽播報句子（預設已帶入真實提醒句子）</label>
                <div class="row">
                  <button type="button" class="btn-ghost" data-preset="垃圾車快到了，約 3 分鐘後到 有謙家園，請準備倒垃圾。">3 分鐘範例</button>
                  <button type="button" class="btn-ghost" data-preset="垃圾車快到了，約 10 分鐘後到 有謙家園，請準備倒垃圾。">10 分鐘範例</button>
                  <button type="button" class="btn-ghost" data-preset="垃圾車已經進入有謙家園社區，請現在出門倒垃圾。">進社區範例</button>
                </div>
              </div>
              <textarea id="broadcast-message">垃圾車快到了，約 3 分鐘後到 有謙家園，請準備倒垃圾。</textarea>
            </div>

            <div class="actions" style="margin-top:14px">
              <button id="broadcast-button" type="button" class="btn-primary" disabled>🔊 立即試聽測試</button>
              <button id="deploy-auto-button" type="button" class="btn-deploy" disabled>🚀 一鍵部署為正式通知設定</button>
            </div>
            <div class="helper" id="broadcast-summary" style="margin-top:8px">正在載入播報設定...</div>
            <div class="message" id="broadcast-result">點「🔊 立即試聽測試」可在 HomePod 試聽；滿意後點「🚀 一鍵部署為正式通知設定」即可永久生效。</div>
          </div>
        </div>
      </div>
    </section>

    <!-- TAB 3: History & ETA Accuracy -->
    <section id="tab-history" class="tab-pane hidden">
      <div class="panel">
        <div class="toolbar">
          <div>
            <h2>歷史資料與「有謙家園」社區繞行軌跡</h2>
            <div class="helper">切換日期即可查看當日完整路線；地圖會用亮橘色特別標出「進入有謙家園後的繞行軌跡」與 80m / 500m 範圍圈。</div>
          </div>
          <div class="row">
            <label for="history-date">選擇清運日期</label>
            <select id="history-date" style="width:auto;min-width:155px"></select>
            <button type="button" class="btn-ghost" id="btn-hist-zoom-target">🎯 聚焦有謙家園 (500m)</button>
            <button type="button" class="btn-ghost" id="btn-hist-zoom-full">🗺️ 顯示完整路線</button>
            <a id="history-json-link" class="btn-ghost" href="#" target="_blank" rel="noreferrer">下載 JSON</a>
            <a id="history-csv-link" class="btn-ghost" href="#" target="_blank" rel="noreferrer">下載 CSV</a>
          </div>
        </div>

        <div class="stats-4" style="margin-bottom:14px">
          <div class="stat"><div class="stat-label">當日 Sample 數 (GPS / 總數)</div><div class="stat-value"><span id="history-gps-count">0</span> / <span id="history-sample-count">0</span></div><div class="stat-sub">Run 狀態：<span id="history-run-status">--</span></div></div>
          <div class="stat"><div class="stat-label">📍 首次抵達有謙家園時間</div><div class="stat-value" id="history-arrival">--</div><div class="stat-sub">已送提醒：<span id="history-notified">--</span></div></div>
          <div class="stat"><div class="stat-label">🏘️ 有謙家園周邊 / 繞行點數</div><div class="stat-value" id="history-loop-count">0 筆</div><div class="stat-sub" id="history-loop-duration">記錄進社區繞行狀態</div></div>
          <div class="stat"><div class="stat-label">收集起訖時間</div><div class="stat-value" style="font-size:.92rem"><span id="history-first">--</span> ～ <span id="history-last">--</span></div><div class="stat-sub">時窗：<span id="history-window">--</span></div></div>
        </div>

        <div class="legend" style="margin-bottom:10px">
          <span><i class="swatch approach"></i> 到站前主路線</span>
          <span><i class="swatch loop"></i> 進入有謙家園 / 社區繞行軌跡 (到站後或 &le;300m)</span>
          <span><i class="swatch c80"></i> 80m 到站判定圈</span>
          <span><i class="swatch c500"></i> 500m 社區繞行圈</span>
        </div>
        <div id="history-map" class="map-box"></div>
        <div class="message" id="history-message">正在載入歷史資料...</div>

        <div class="toolbar" style="margin-top:16px;margin-bottom:6px">
          <h3>當日 GPS 軌跡點明細（含距離有謙家園公尺數）</h3>
          <div class="row">
            <button type="button" class="btn-ghost active" id="filter-all-btn">全部軌跡點</button>
            <button type="button" class="btn-ghost" id="filter-loop-btn">🏘️ 只看有謙家園周邊 (&le;500m) 與到站後繞行</button>
          </div>
        </div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>時間</th><th>階段</th><th>距有謙家園</th><th>GPS 座標</th><th>路線進度</th><th>官方 API 預估</th><th>Waiting</th></tr></thead>
            <tbody id="history-table-body"><tr><td colspan="7" class="helper">尚未載入資料。</td></tr></tbody>
          </table>
        </div>
      </div>

      <!-- ETA Accuracy Section -->
      <div class="panel" style="margin-top:14px">
        <div class="toolbar">
          <div>
            <h2>ETA 評估（歷史提醒準確度回算）</h2>
            <div class="helper">依每次實際抵達有謙家園的時間 (`arrival_at`)，自動回算 10 分鐘與 3 分鐘提醒的實際提前量與誤差。</div>
          </div>
          <a id="eta-eval-link" class="btn-ghost" href="./api/eta/evaluation" target="_blank" rel="noreferrer">檢視評估 JSON</a>
        </div>
        <div class="stats-4">
          <div class="stat"><div class="stat-label">已評估提醒總數</div><div class="stat-value" id="eta-alert-count">0</div></div>
          <div class="stat"><div class="stat-label">平均絕對誤差</div><div class="stat-value" id="eta-avg-error">--</div></div>
          <div class="stat"><div class="stat-label">歷史模型命中次數</div><div class="stat-value" id="eta-historical-count">0</div></div>
          <div class="stat"><div class="stat-label">API 備援次數</div><div class="stat-value" id="eta-api-count">0</div></div>
        </div>
        <div class="grid" style="margin-top:12px">
          <div class="s6"><h3>依預測來源表現</h3><div class="table-wrap"><table><thead><tr><th>來源</th><th>次數</th><th>平均提前</th><th>平均誤差</th><th>平均絕對誤差</th></tr></thead><tbody id="eta-source-body"><tr><td colspan="5" class="helper">尚未載入。</td></tr></tbody></table></div></div>
          <div class="s6"><h3>依提醒門檻表現</h3><div class="table-wrap"><table><thead><tr><th>門檻</th><th>次數</th><th>平均提前</th><th>平均誤差</th><th>平均絕對誤差</th></tr></thead><tbody id="eta-offset-body"><tr><td colspan="5" class="helper">尚未載入。</td></tr></tbody></table></div></div>
        </div>
        <details style="margin-top:12px">
          <summary>📋 展開歷次提醒誤差明細表</summary>
          <div class="table-wrap">
            <table>
              <thead><tr><th>日期</th><th>門檻</th><th>預測來源</th><th>信心</th><th>提醒送出時間</th><th>實際到站時間</th><th>實際提前</th><th>誤差</th></tr></thead>
              <tbody id="eta-row-body"><tr><td colspan="8" class="helper">尚未載入 ETA 評估。</td></tr></tbody>
            </table>
          </div>
        </details>
      </div>
    </section>
  </main>

  <script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js" crossorigin=""></script>
  <script>
    const statusURL=new URL("./status",window.location.href),
          historyDatesURL=new URL("./api/history/dates",window.location.href),
          historyTodayURL=new URL("./api/history/today",window.location.href),
          historyDayURL=new URL("./api/history/day",window.location.href),
          historyDayJSONURL=new URL("./api/history/day.json",window.location.href),
          historyDayCSVURL=new URL("./api/history/day.csv",window.location.href),
          etaEvaluationURL=new URL("./api/eta/evaluation",window.location.href),
          broadcastOptionsURL=new URL("./api/broadcast/options",window.location.href),
          broadcastTestURL=new URL("./api/broadcast/test",window.location.href),
          broadcastAutoURL=new URL("./api/broadcast/auto",window.location.href);

    const DEFAULT_TARGET=[24.748448,121.02032];
    const els={
      routeTitle:document.getElementById("route-title"),
      routeSubtitle:document.getElementById("route-subtitle"),
      serviceBadge:document.getElementById("service-badge"),
      predictionBadge:document.getElementById("prediction-badge"),
      remainingMinutes:document.getElementById("remaining-minutes"),
      arrivalAt:document.getElementById("arrival-at"),
      topDistance:document.getElementById("top-distance"),
      predictionSource:document.getElementById("prediction-source"),
      predictionConfidence:document.getElementById("prediction-confidence"),
      todaySamples:document.getElementById("today-samples"),
      todayGPSSamples:document.getElementById("today-gps-samples"),
      notifiedOffsets:document.getElementById("notified-offsets"),
      statusMessage:document.getElementById("status-message"),
      collectionWindow:document.getElementById("collection-window"),
      stepWindow:document.getElementById("step-window"),
      stepGPS:document.getElementById("step-gps"),
      stepGPSMeta:document.getElementById("step-gps-meta"),
      stepAlert10:document.getElementById("step-alert10"),
      stepAlert10Meta:document.getElementById("step-alert10-meta"),
      stepAlert3:document.getElementById("step-alert3"),
      stepAlert3Meta:document.getElementById("step-alert3-meta"),
      stepLoop:document.getElementById("step-loop"),
      stepLoopMeta:document.getElementById("step-loop-meta"),
      stepDone:document.getElementById("step-done"),
      stepDoneMeta:document.getElementById("step-done-meta"),
      liveDistance:document.getElementById("live-distance"),
      gpsStatus:document.getElementById("gps-status"),
      apiEstimated:document.getElementById("api-estimated"),
      apiWaiting:document.getElementById("api-waiting"),
      truckCoords:document.getElementById("truck-coords"),
      mapLink:document.getElementById("map-link"),
      lastCollected:document.getElementById("last-collected"),
      updatedAt:document.getElementById("updated-at"),
      liveMapBanner:document.getElementById("live-map-banner"),
      serviceDate:document.getElementById("service-date"),
      weekday:document.getElementById("weekday"),
      runStatus:document.getElementById("run-status"),
      targetDays:document.getElementById("target-days"),
      checkInterval:document.getElementById("check-interval"),
      targetCoords:document.getElementById("target-coords"),
      sharedDataPath:document.getElementById("shared-data-path"),
      statusLink:document.getElementById("status-link"),
      rawJSON:document.getElementById("raw-json"),
      /* Unified Broadcast Studio */
      diffBadge:document.getElementById("diff-badge"),
      depEngine:document.getElementById("dep-engine"),
      depVoice:document.getElementById("dep-voice"),
      depParams:document.getElementById("dep-params"),
      depTargets:document.getElementById("dep-targets"),
      resetToDeployedBtn:document.getElementById("reset-to-deployed-btn"),
      ttsEntity:document.getElementById("tts-entity"),
      ttsLanguage:document.getElementById("tts-language"),
      voiceTierSection:document.getElementById("voice-tier-section"),
      tierPills:document.getElementById("tier-pills"),
      ttsVoice:document.getElementById("tts-voice"),
      ttsCloudWrap:document.getElementById("tts-cloud-wrap"),
      chirpHint:document.getElementById("chirp-hint"),
      prosodyControls:document.getElementById("prosody-controls"),
      ttsInputMode:document.getElementById("tts-input-mode"),
      ttsSpeakingRate:document.getElementById("tts-speaking-rate"),
      ttsPitch:document.getElementById("tts-pitch"),
      ttsVolumeGainDB:document.getElementById("tts-volume-gain-db"),
      ttsEffectsProfile:document.getElementById("tts-effects-profile"),
      deviceList:document.getElementById("device-list"),
      broadcastMessage:document.getElementById("broadcast-message"),
      broadcastButton:document.getElementById("broadcast-button"),
      deployAutoButton:document.getElementById("deploy-auto-button"),
      broadcastSummary:document.getElementById("broadcast-summary"),
      broadcastResult:document.getElementById("broadcast-result"),
      /* History & ETA */
      historyDate:document.getElementById("history-date"),
      historyJSONLink:document.getElementById("history-json-link"),
      historyCSVLink:document.getElementById("history-csv-link"),
      historySampleCount:document.getElementById("history-sample-count"),
      historyGPSCount:document.getElementById("history-gps-count"),
      historyFirst:document.getElementById("history-first"),
      historyLast:document.getElementById("history-last"),
      historyRunStatus:document.getElementById("history-run-status"),
      historyNotified:document.getElementById("history-notified"),
      historyArrival:document.getElementById("history-arrival"),
      historyLoopCount:document.getElementById("history-loop-count"),
      historyLoopDuration:document.getElementById("history-loop-duration"),
      historyWindow:document.getElementById("history-window"),
      historyMessage:document.getElementById("history-message"),
      historyTableBody:document.getElementById("history-table-body"),
      filterAllBtn:document.getElementById("filter-all-btn"),
      filterLoopBtn:document.getElementById("filter-loop-btn"),
      etaEvalLink:document.getElementById("eta-eval-link"),
      etaAlertCount:document.getElementById("eta-alert-count"),
      etaAvgError:document.getElementById("eta-avg-error"),
      etaHistoricalCount:document.getElementById("eta-historical-count"),
      etaAPICount:document.getElementById("eta-api-count"),
      etaSourceBody:document.getElementById("eta-source-body"),
      etaOffsetBody:document.getElementById("eta-offset-body"),
      etaRowBody:document.getElementById("eta-row-body")
    };

    let selectedHistoryDate="",
        currentHistoryDay=null,
        historyTableFilter="all",
        liveMap,liveLayer,liveFullBounds=[],
        historyMap,historyLayer,historyFullBounds=[],
        currentStatus=null,
        selectedVoiceTier="WaveNet",
        broadcastOptions={media_players:[],tts_entities:[],default_tts_entity:"",google_cloud_voices:[]},
        deployedSettings=null;

    const fmt=(v)=>{
      if(!v)return"--";
      const d=new Date(v);
      return Number.isNaN(d.getTime())?v:new Intl.DateTimeFormat("zh-TW",{month:"2-digit",day:"2-digit",hour:"2-digit",minute:"2-digit",second:"2-digit",hour12:false}).format(d);
    };
    const fmtShort=(v)=>{
      if(!v)return"--";
      const d=new Date(v);
      return Number.isNaN(d.getTime())?v:new Intl.DateTimeFormat("zh-TW",{hour:"2-digit",minute:"2-digit",second:"2-digit",hour12:false}).format(d);
    };
    const coords=(a,b)=>typeof a==="number"&&typeof b==="number"&&(a!==0||b!==0)?a.toFixed(6)+", "+b.toFixed(6):"--";
    const label=(m,k)=>m[k]||k||"--";
    const runStatusText=(st)=>label({collecting:"收集中",post_arrival:"已抵有謙家園・繞行記錄中",completed:"已完成"},st);

    function haversineMeters(lat1,lng1,lat2,lng2){
      if(typeof lat1!=="number"||typeof lng1!=="number"||typeof lat2!=="number"||typeof lng2!=="number")return null;
      if((lat1===0&&lng1===0)||(lat2===0&&lng2===0))return null;
      const R=6371000,toRad=(x)=>x*Math.PI/180,dLat=toRad(lat2-lat1),dLng=toRad(lng2-lng1);
      const a=Math.sin(dLat/2)**2+Math.cos(toRad(lat1))*Math.cos(toRad(lat2))*Math.sin(dLng/2)**2;
      return 2*R*Math.atan2(Math.sqrt(a),Math.sqrt(1-a));
    }
    function fmtDistance(m){
      if(m===null||m===undefined||Number.isNaN(m))return"--";
      return m>=1000?(m/1000).toFixed(2)+" km":Math.round(m)+" m";
    }
    function googleMapURL(a,b,c,d){
      const p=new URLSearchParams({api:"1",origin:a+","+b,destination:c+","+d,travelmode:"driving"});
      return "https://www.google.com/maps/dir/?"+p.toString();
    }

    /* Tab Switching */
    document.querySelectorAll(".tab-btn").forEach((btn)=>{
      btn.addEventListener("click",()=>{
        const targetID=btn.getAttribute("data-tab");
        document.querySelectorAll(".tab-btn").forEach((b)=>b.classList.toggle("active",b===btn));
        document.querySelectorAll(".tab-pane").forEach((pane)=>pane.classList.toggle("hidden",pane.id!==targetID));
        window.setTimeout(()=>{
          if(targetID==="tab-live"&&liveMap)liveMap.invalidateSize();
          if(targetID==="tab-history"&&historyMap)historyMap.invalidateSize();
        },60);
      });
    });

    /* Map Rendering (Dual-Color Approach vs Community Loop) */
    function initMapInstance(containerID){
      const m=L.map(containerID);
      L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png",{maxZoom:19,attribution:"&copy; OpenStreetMap"}).addTo(m);
      const layer=L.layerGroup().addTo(m);
      m.setView(DEFAULT_TARGET,15);
      return {map:m,layer};
    }
    function ensureLiveMap(){
      if(liveMap)return;
      const res=initMapInstance("live-map");
      liveMap=res.map;liveLayer=res.layer;
    }
    function ensureHistoryMap(){
      if(historyMap)return;
      const res=initMapInstance("history-map");
      historyMap=res.map;historyLayer=res.layer;
    }
    function isLoopSample(sample,arrivalAtMs,targetLat,targetLng){
      if(arrivalAtMs&&new Date(sample.collected_at).getTime()>=arrivalAtMs)return true;
      const dist=haversineMeters(sample.truck_lat,sample.truck_lng,targetLat,targetLng);
      return dist!==null&&dist<=300;
    }
    function drawTrajectoryOnLayer(mapObj,layer,day,liveTruck){
      layer.clearLayers();
      const targetLat=typeof day?.target_lat==="number"&&day.target_lat!==0?day.target_lat:DEFAULT_TARGET[0];
      const targetLng=typeof day?.target_lng==="number"&&day.target_lng!==0?day.target_lng:DEFAULT_TARGET[1];
      const targetPt=[targetLat,targetLng];
      const bounds=[targetPt];

      // 500m community departure circle & 80m arrival radius circle
      L.circle(targetPt,{radius:500,color:"#5bc0be",weight:1.5,dashArray:"6 6",fillColor:"#5bc0be",fillOpacity:.04}).addTo(layer).bindPopup("有謙家園 500m 社區繞行範圍圈");
      L.circle(targetPt,{radius:80,color:"#ffb347",weight:2,dashArray:"4 4",fillColor:"#ffb347",fillOpacity:.12}).addTo(layer).bindPopup("有謙家園 80m 到站判定圈");
      L.circleMarker(targetPt,{radius:8,color:"#ffd36b",fillColor:"#ff9f1c",fillOpacity:.95,weight:2}).addTo(layer).bindPopup("🎯 "+(day?.point_name||"有謙家園")+"（第 "+(day?.point_seq||27)+" 站）");

      const gps=(day?.samples||[]).filter((s)=>s.gps_available&&typeof s.truck_lat==="number"&&typeof s.truck_lng==="number"&&(s.truck_lat!==0||s.truck_lng!==0));
      const arrivalMs=day?.arrival_at?new Date(day.arrival_at).getTime():null;

      if(gps.length){
        const approachPts=[];
        const loopPts=[];
        let firstLoopSample=null;
        gps.forEach((s)=>{
          const pt=[s.truck_lat,s.truck_lng];
          bounds.push(pt);
          if(isLoopSample(s,arrivalMs,targetLat,targetLng)){
            if(!firstLoopSample){
              firstLoopSample=s;
              if(approachPts.length)loopPts.push(approachPts[approachPts.length-1]);
            }
            loopPts.push(pt);
          }else{
            approachPts.push(pt);
          }
        });

        if(approachPts.length>1){
          L.polyline(approachPts,{color:"#62d6b0",weight:4,opacity:.88}).addTo(layer);
        }
        if(loopPts.length>1){
          L.polyline(loopPts,{color:"#ffb347",weight:6,opacity:.96}).addTo(layer);
        }else if(approachPts.length&&loopPts.length===1){
          L.polyline([approachPts[approachPts.length-1],loopPts[0]],{color:"#ffb347",weight:6,opacity:.96}).addTo(layer);
        }

        const first=gps[0],last=gps[gps.length-1];
        L.circleMarker([first.truck_lat,first.truck_lng],{radius:6,color:"#62d6b0",fillColor:"#061015",fillOpacity:1,weight:3}).addTo(layer).bindPopup("🟢 起點 ("+fmtShort(first.collected_at)+")");
        if(firstLoopSample){
          L.circleMarker([firstLoopSample.truck_lat,firstLoopSample.truck_lng],{radius:7,color:"#ffb347",fillColor:"#ffb347",fillOpacity:1}).addTo(layer).bindPopup("🏘️ 進入有謙家園周邊 ("+fmtShort(firstLoopSample.collected_at)+")");
        }
        L.marker([last.truck_lat,last.truck_lng]).addTo(layer).bindPopup("🏁 最後紀錄點 ("+fmtShort(last.collected_at)+")");
      }

      if(liveTruck&&liveTruck.gps_available&&typeof liveTruck.truck_lat==="number"&&typeof liveTruck.truck_lng==="number"){
        const pt=[liveTruck.truck_lat,liveTruck.truck_lng];
        bounds.push(pt);
        const dist=haversineMeters(liveTruck.truck_lat,liveTruck.truck_lng,targetLat,targetLng);
        L.circleMarker(pt,{radius:9,color:"#ffffff",fillColor:"#5ce0b8",fillOpacity:1,weight:3}).addTo(layer).bindPopup("🚛 垃圾車目前位置（距有謙家園 "+fmtDistance(dist)+"）");
      }

      if(bounds.length>1){
        mapObj.fitBounds(bounds,{padding:[28,28]});
      }else{
        mapObj.setView(targetPt,16);
      }
      return bounds;
    }

    document.getElementById("btn-live-zoom-target").addEventListener("click",()=>{if(liveMap)liveMap.setView(DEFAULT_TARGET,16)});
    document.getElementById("btn-live-zoom-full").addEventListener("click",()=>{if(liveMap&&liveFullBounds.length>1)liveMap.fitBounds(liveFullBounds,{padding:[28,28]})});
    document.getElementById("btn-hist-zoom-target").addEventListener("click",()=>{if(historyMap)historyMap.setView(DEFAULT_TARGET,16)});
    document.getElementById("btn-hist-zoom-full").addEventListener("click",()=>{if(historyMap&&historyFullBounds.length>1)historyMap.fitBounds(historyFullBounds,{padding:[28,28]})});

    /* Render Live Status & Stepper */
    function setStepState(el,state){
      el.className="step"+(state?" "+state:"");
    }
    function renderStatus(s){
      currentStatus=s;
      const p=s.prediction;
      const targetLat=typeof s.target_lat==="number"&&s.target_lat!==0?s.target_lat:DEFAULT_TARGET[0];
      const targetLng=typeof s.target_lng==="number"&&s.target_lng!==0?s.target_lng:DEFAULT_TARGET[1];
      const dist=s.gps_available?haversineMeters(s.truck_lat,s.truck_lng,targetLat,targetLng):null;

      els.routeTitle.textContent=s.route_name||"雙溪線";
      els.routeSubtitle.textContent=(s.point_name||"有謙家園")+"（第 "+(s.point_seq||27)+" 站）";

      if(s.run_status==="post_arrival"){
        els.serviceBadge.textContent="🏘️ 社區繞行記錄中";
        els.serviceBadge.className="badge loop";
      }else if(s.active){
        els.serviceBadge.textContent="🟢 清運收集中";
        els.serviceBadge.className="badge active";
      }else if(s.run_status==="completed"){
        els.serviceBadge.textContent="✓ 今日已完成";
        els.serviceBadge.className="badge active";
      }else{
        els.serviceBadge.textContent="待命中";
        els.serviceBadge.className="badge inactive";
      }

      if(p){
        const srcText=label({historical_model:"歷史模型",api_estimated_time:"API 預估時間",api_waiting_time:"API 等待時間"},p.source);
        els.predictionBadge.textContent=srcText;
        els.predictionBadge.className="badge active";
        els.remainingMinutes.textContent=p.remaining_minutes+" 分鐘";
        els.arrivalAt.textContent="預計 "+fmtShort(p.predicted_arrival_at)+" 到站";
        els.predictionSource.textContent=srcText;
        els.predictionConfidence.textContent=label({high:"高信心",medium:"中信心",low:"低信心"},p.confidence);
      }else if(s.run_status==="post_arrival"){
        els.predictionBadge.textContent="已抵達有謙家園";
        els.predictionBadge.className="badge loop";
        els.remainingMinutes.textContent="已到站";
        els.arrivalAt.textContent="正在記錄有謙家園繞行路線";
        els.predictionSource.textContent="社區繞行";
        els.predictionConfidence.textContent="--";
      }else{
        els.predictionBadge.textContent="等待預測";
        els.predictionBadge.className="badge inactive";
        els.remainingMinutes.textContent="--";
        els.arrivalAt.textContent=s.active?"收集軌跡中，等待模型計算":"目前非清運時段";
        els.predictionSource.textContent="待命中";
        els.predictionConfidence.textContent="--";
      }

      els.topDistance.textContent="距有謙家園："+fmtDistance(dist);
      els.liveDistance.textContent=fmtDistance(dist);
      els.todaySamples.textContent=String(s.today_sample_count||0);
      els.todayGPSSamples.textContent=String(s.today_gps_sample_count||0);
      els.statusMessage.textContent=s.message||"服務執行中";
      els.serviceDate.textContent=s.service_date||"--";
      els.weekday.textContent=s.weekday||"--";
      els.collectionWindow.textContent=s.collection_window||"19:00-21:30";
      els.runStatus.textContent=runStatusText(s.run_status);
      els.lastCollected.textContent=fmtShort(s.last_collected_at);
      const offsets=s.notified_offsets||[];
      els.notifiedOffsets.textContent=offsets.length?offsets.map((o)=>o+"分").join(" / "):"尚未送出";
      els.targetDays.textContent=(s.target_days||[]).join(" / ")||"--";
      els.checkInterval.textContent=s.check_interval||"--";
      els.sharedDataPath.textContent=s.shared_data_path||"--";
      els.updatedAt.textContent=fmtShort(s.updated_at);
      els.gpsStatus.textContent=s.gps_available?"GPS 正常":"目前無 GPS";
      els.truckCoords.textContent=s.gps_available?coords(s.truck_lat,s.truck_lng):"--";
      els.targetCoords.textContent=coords(targetLat,targetLng);
      els.apiEstimated.textContent=s.api_estimated_text||"--";
      els.apiWaiting.textContent=s.api_waiting_time===null||s.api_waiting_time===undefined?"--":String(s.api_waiting_time);
      els.rawJSON.textContent=JSON.stringify(s,null,2);
      els.statusLink.href=statusURL.toString();

      if(s.gps_available){
        els.mapLink.href=googleMapURL(s.truck_lat,s.truck_lng,targetLat,targetLng);
        els.mapLink.textContent="開啟 Google Maps 路線";
      }else{
        els.mapLink.href="#";
        els.mapLink.textContent="等待 GPS 座標";
      }

      // Update 6-step timeline
      const hasSamples=(s.today_sample_count||0)>0;
      const sent10=offsets.includes(10);
      const sent3=offsets.includes(3);
      const isLoop=s.run_status==="post_arrival";
      const isDone=s.run_status==="completed";

      setStepState(els.stepWindow,(s.active||hasSamples||isDone)?"done":"");
      setStepState(els.stepGPS,(isLoop||isDone)?"done":(s.active&&hasSamples?"current":(hasSamples?"done":"")));
      els.stepGPSMeta.textContent=hasSamples?(s.today_gps_sample_count+" 筆 GPS"):"尚未發車";
      setStepState(els.stepAlert10,sent10?"done":"");
      els.stepAlert10Meta.textContent=sent10?"✓ 已發送":"未觸發";
      setStepState(els.stepAlert3,sent3?"done":"");
      els.stepAlert3Meta.textContent=sent3?"✓ 已發送":"未觸發";
      setStepState(els.stepLoop,isDone?"done":(isLoop?"current":""));
      els.stepLoopMeta.textContent=isLoop?"🏘️ 繞行記錄中":(isDone?"✓ 已完成繞行":"等待進站");
      setStepState(els.stepDone,isDone?"done":"");
      els.stepDoneMeta.textContent=isDone?"✓ 今日已完成":"等待結束";
    }

    /* History & Live Map Loading */
    function updateHistoryLinks(date){
      const j=new URL(historyDayJSONURL),c=new URL(historyDayCSVURL);
      j.searchParams.set("date",date);c.searchParams.set("date",date);
      els.historyJSONLink.href=j.toString();els.historyCSVLink.href=c.toString();
    }
    function renderHistoryTable(day){
      const samples=day?.samples||[];
      const targetLat=typeof day?.target_lat==="number"&&day.target_lat!==0?day.target_lat:DEFAULT_TARGET[0];
      const targetLng=typeof day?.target_lng==="number"&&day.target_lng!==0?day.target_lng:DEFAULT_TARGET[1];
      const arrivalMs=day?.arrival_at?new Date(day.arrival_at).getTime():null;

      const rows=[];
      samples.forEach((s)=>{
        const dist=s.gps_available?haversineMeters(s.truck_lat,s.truck_lng,targetLat,targetLng):null;
        const inLoop=isLoopSample(s,arrivalMs,targetLat,targetLng)||(dist!==null&&dist<=500);
        if(historyTableFilter==="loop"&&!inLoop)return;
        const phaseBadge=inLoop?"<span class='badge loop' style='padding:2px 8px;font-size:.76rem'>🏘️ 社區周邊/繞行</span>":"<span class='muted'>到站前路線</span>";
        rows.push(
          "<tr class='"+(inLoop?"loop-row":"")+"'>"+
            "<td>"+fmtShort(s.collected_at)+"</td>"+
            "<td>"+phaseBadge+"</td>"+
            "<td><strong>"+fmtDistance(dist)+"</strong></td>"+
            "<td>"+(s.gps_available?coords(s.truck_lat,s.truck_lng):"--")+"</td>"+
            "<td>"+(s.progress_meters===null||s.progress_meters===undefined?"--":s.progress_meters.toFixed(0)+" m")+"</td>"+
            "<td>"+(s.api_estimated_minutes===null||s.api_estimated_minutes===undefined?(s.api_estimated_text||"--"):s.api_estimated_minutes+" 分")+"</td>"+
            "<td>"+(s.api_waiting_time===null||s.api_waiting_time===undefined?"--":s.api_waiting_time)+"</td>"+
          "</tr>"
        );
      });

      els.historyTableBody.innerHTML=rows.length?rows.join(""):"<tr><td colspan='7' class='helper'>沒有符合篩選條件的軌跡點。</td></tr>";
    }
    function renderHistory(day){
      currentHistoryDay=day;
      updateHistoryLinks(day.service_date);
      els.historySampleCount.textContent=String(day.sample_count||0);
      els.historyGPSCount.textContent=String(day.gps_sample_count||0);
      els.historyFirst.textContent=fmtShort(day.first_collected_at);
      els.historyLast.textContent=fmtShort(day.last_collected_at);
      els.historyRunStatus.textContent=runStatusText(day.run_status);
      els.historyNotified.textContent=(day.notified_offsets||[]).length?day.notified_offsets.map((o)=>o+"分").join(" / "):"無";
      els.historyArrival.textContent=day.arrival_at?fmtShort(day.arrival_at):"尚未紀錄到站";
      els.historyWindow.textContent=day.collection_window||"19:00-21:30";

      const targetLat=typeof day.target_lat==="number"&&day.target_lat!==0?day.target_lat:DEFAULT_TARGET[0];
      const targetLng=typeof day.target_lng==="number"&&day.target_lng!==0?day.target_lng:DEFAULT_TARGET[1];
      const arrivalMs=day.arrival_at?new Date(day.arrival_at).getTime():null;
      const loopSamples=(day.samples||[]).filter((s)=>s.gps_available&&(isLoopSample(s,arrivalMs,targetLat,targetLng)||(haversineMeters(s.truck_lat,s.truck_lng,targetLat,targetLng)||9999)<=500));
      els.historyLoopCount.textContent=loopSamples.length+" 筆";
      if(loopSamples.length>=2){
        const mins=Math.max(1,Math.round((new Date(loopSamples[loopSamples.length-1].collected_at)-new Date(loopSamples[0].collected_at))/60000));
        els.historyLoopDuration.textContent="社區周邊涵蓋約 "+mins+" 分鐘";
      }else{
        els.historyLoopDuration.textContent=loopSamples.length===1?"僅記錄到進站瞬間（舊版紀錄）":"當日未進入 500m 範圍";
      }

      els.historyMessage.textContent=(day.samples||[]).length?("已載入 "+day.service_date+" 的收集軌跡（共 "+day.sample_count+" 筆）。"):"這一天沒有收集到軌跡點。";
      ensureHistoryMap();
      historyFullBounds=drawTrajectoryOnLayer(historyMap,historyLayer,day,null);
      renderHistoryTable(day);
    }

    els.filterAllBtn.addEventListener("click",()=>{
      historyTableFilter="all";
      els.filterAllBtn.classList.add("active");
      els.filterLoopBtn.classList.remove("active");
      if(currentHistoryDay)renderHistoryTable(currentHistoryDay);
    });
    els.filterLoopBtn.addEventListener("click",()=>{
      historyTableFilter="loop";
      els.filterLoopBtn.classList.add("active");
      els.filterAllBtn.classList.remove("active");
      if(currentHistoryDay)renderHistoryTable(currentHistoryDay);
    });

    async function loadHistoryDay(date){
      selectedHistoryDate=date;
      updateHistoryLinks(date);
      const u=new URL(historyDayURL);
      u.searchParams.set("date",date);
      try{
        const r=await fetch(u,{cache:"no-store"});
        if(!r.ok)throw new Error("HTTP "+r.status);
        const day=await r.json();
        renderHistory(day);
        return day;
      }catch(e){
        els.historyMessage.textContent="歷史資料讀取失敗："+e.message;
        els.historyMessage.className="message error";
        return null;
      }
    }

    async function loadHistoryDatesAndLiveMap(){
      const [dr,tr]=await Promise.all([fetch(historyDatesURL,{cache:"no-store"}),fetch(historyTodayURL,{cache:"no-store"})]);
      if(!dr.ok)throw new Error("dates HTTP "+dr.status);
      if(!tr.ok)throw new Error("today HTTP "+tr.status);
      const dp=await dr.json(),today=await tr.json(),dates=dp.dates||[];
      const preferredHistoryDate=(today.samples&&today.samples.length)?today.service_date:(dates[0]||today.service_date);

      els.historyDate.innerHTML="";
      Array.from(new Set([...dates,today.service_date])).filter(Boolean).forEach((d)=>{
        const o=document.createElement("option");
        o.value=d;o.textContent=d;o.selected=d===preferredHistoryDate;
        els.historyDate.appendChild(o);
      });

      selectedHistoryDate=preferredHistoryDate;
      let histDay=today;
      if(preferredHistoryDate!==today.service_date){
        histDay=await loadHistoryDay(preferredHistoryDate)||today;
      }else{
        renderHistory(today);
      }

      // Render Live Map (Tab 1): use today if it has GPS samples, otherwise preview most recent historical day!
      ensureLiveMap();
      if(today.samples&&today.samples.length){
        els.liveMapBanner.textContent="🟢 即時顯示今日 ("+today.service_date+") 清運軌跡（共 "+today.sample_count+" 筆）";
        liveFullBounds=drawTrajectoryOnLayer(liveMap,liveLayer,today,currentStatus);
      }else if(histDay&&histDay.samples&&histDay.samples.length){
        els.liveMapBanner.textContent="🕒 今日尚未發車・目前地圖預覽最近一次清運日 ("+histDay.service_date+") 軌跡";
        liveFullBounds=drawTrajectoryOnLayer(liveMap,liveLayer,histDay,currentStatus);
      }else{
        els.liveMapBanner.textContent="目前尚無軌跡資料，已標出有謙家園目標位置與 80m / 500m 範圍圈";
        liveFullBounds=drawTrajectoryOnLayer(liveMap,liveLayer,today,currentStatus);
      }
    }

    /* ETA Evaluation */
    function renderBucketRows(rows){
      return (rows||[]).length?(rows||[]).map((r)=>"<tr><td>"+r.name+"</td><td>"+r.count+"</td><td>"+r.average_lead_minutes+" 分</td><td>"+r.average_error_minutes+" 分</td><td>"+r.average_abs_error_minutes+" 分</td></tr>").join(""):"<tr><td colspan='5' class='helper'>沒有可評估資料。</td></tr>";
    }
    function renderETAEvaluation(e){
      const s=e.summary||{};
      els.etaEvalLink.href=etaEvaluationURL.toString();
      els.etaAlertCount.textContent=String(s.alert_count||0);
      els.etaAvgError.textContent=s.average_abs_error_minutes===undefined?"--":s.average_abs_error_minutes+" 分";
      els.etaHistoricalCount.textContent=String(s.historical_model_count||0);
      els.etaAPICount.textContent=String(s.api_fallback_count||0);
      els.etaSourceBody.innerHTML=renderBucketRows(e.by_source);
      els.etaOffsetBody.innerHTML=renderBucketRows(e.by_offset);
      els.etaRowBody.innerHTML=(e.rows||[]).length?(e.rows||[]).map((r)=>"<tr><td>"+r.service_date+"</td><td>"+r.offset_minutes+" 分</td><td>"+label({historical_model:"歷史模型",api_estimated_time:"API 預估",api_waiting_time:"API 等待"},r.prediction_source)+"</td><td>"+(r.confidence||"--")+"</td><td>"+fmtShort(r.sent_at)+"</td><td>"+fmtShort(r.arrival_at)+"</td><td>"+r.lead_minutes+" 分</td><td>"+r.error_minutes+" 分</td></tr>").join(""):"<tr><td colspan='8' class='helper'>沒有可評估提醒。</td></tr>";
    }
    async function loadETAEvaluation(){
      try{
        const r=await fetch(etaEvaluationURL,{cache:"no-store"});
        if(!r.ok)throw new Error("HTTP "+r.status);
        renderETAEvaluation(await r.json());
      }catch(e){
        els.etaRowBody.innerHTML="<tr><td colspan='8' class='helper'>ETA 評估讀取失敗："+e.message+"</td></tr>";
      }
    }

    /* Unified HomePod Broadcast Studio */
    function isGoogleCloudEntity(entityID){return entityID==="tts.google_cloud_tts_direct"}
    function isChirpVoice(voiceName){return (voiceName||"").toLowerCase().includes("chirp3-hd")}
    function inferVoiceTier(voiceName){
      const v=(voiceName||"").toLowerCase();
      if(v.includes("chirp3-hd"))return "Chirp 3: HD";
      if(v.includes("neural2"))return "Neural2";
      if(v.includes("wavenet"))return "WaveNet";
      return "WaveNet";
    }
    function selectedTargets(){
      return Array.from(document.querySelectorAll("input[name='broadcast-target']:checked")).map((n)=>n.value);
    }
    function friendlyGender(g){
      if(g==="FEMALE")return "♀ 女聲";
      if(g==="MALE")return "♂ 男聲";
      return g||"--";
    }
    function renderTTSEntitySelect(selectedEntityID){
      els.ttsEntity.innerHTML="";
      const entities=broadcastOptions.tts_entities||[];
      if(!entities.length){
        const o=document.createElement("option");o.value="";o.textContent="找不到可用 TTS";els.ttsEntity.appendChild(o);return;
      }
      const targetID=selectedEntityID||broadcastOptions.default_tts_entity||entities[0].entity_id;
      entities.forEach((entity)=>{
        const o=document.createElement("option");
        o.value=entity.entity_id;
        o.textContent=entity.friendly_name+" ("+entity.entity_id+")";
        o.selected=entity.entity_id===targetID;
        els.ttsEntity.appendChild(o);
      });
    }
    function renderVoiceOptions(selectedVoice){
      const entityID=els.ttsEntity.value;
      els.ttsVoice.innerHTML="";
      if(!isGoogleCloudEntity(entityID)){
        const o=document.createElement("option");
        o.value="";
        o.textContent="Google Translate 固定台灣中文語音（無須選擇聲線）";
        els.ttsVoice.appendChild(o);
        return;
      }
      const allVoices=broadcastOptions.google_cloud_voices||[];
      const filtered=selectedVoiceTier==="ALL"?allVoices:allVoices.filter((v)=>(v.tier||inferVoiceTier(v.name))===selectedVoiceTier);
      const list=filtered.length?filtered:allVoices;
      const effective=selectedVoice||(list[0]?list[0].name:"cmn-TW-Wavenet-A");

      list.forEach((voice)=>{
        const o=document.createElement("option");
        const tier=voice.tier||inferVoiceTier(voice.name);
        const locale=voice.name.startsWith("cmn-TW")?"台灣口音":"中文";
        o.value=voice.name;
        o.textContent="["+tier+"] "+voice.name+" · "+friendlyGender(voice.ssml_gender)+" ("+locale+")";
        o.selected=voice.name===effective;
        els.ttsVoice.appendChild(o);
      });
    }
    function syncTierPillHighlight(){
      Array.from(els.tierPills.querySelectorAll(".tier-pill")).forEach((btn)=>{
        btn.classList.toggle("active",btn.getAttribute("data-tier")===selectedVoiceTier);
      });
    }
    function syncBroadcastFormUI(preferredVoice){
      const entityID=els.ttsEntity.value;
      const isCloud=isGoogleCloudEntity(entityID);
      els.voiceTierSection.classList.toggle("hidden",!isCloud);
      els.ttsCloudWrap.classList.toggle("hidden",!isCloud);

      const langLower=els.ttsLanguage.value.trim().toLowerCase();
      if(isCloud){
        if(!langLower||langLower==="zh-tw"||langLower==="cmn-tw")els.ttsLanguage.value="cmn-TW";
      }else{
        els.ttsLanguage.value="zh-tw";
      }

      renderVoiceOptions(preferredVoice||els.ttsVoice.value);
      const currentVoice=els.ttsVoice.value;
      const chirp=isCloud&&isChirpVoice(currentVoice);
      els.chirpHint.classList.toggle("hidden",!chirp);
      els.prosodyControls.classList.toggle("hidden",chirp);
      updateStudioState();
    }
    function renderDeviceCheckboxes(selectedIDs){
      els.deviceList.innerHTML="";
      const players=broadcastOptions.media_players||[];
      if(!players.length){
        els.deviceList.innerHTML="<div class='helper'>目前找不到可用的 HomePod 或 media_player。</div>";
        return;
      }
      const selectedSet=new Set(selectedIDs||[]);
      players.forEach((device)=>{
        const lbl=document.createElement("label");
        lbl.className="device";
        const input=document.createElement("input");
        input.type="checkbox";
        input.name="broadcast-target";
        input.value=device.entity_id;
        input.checked=selectedSet.size?selectedSet.has(device.entity_id):true;
        input.addEventListener("change",updateStudioState);
        const meta=document.createElement("div");
        meta.innerHTML="<strong>"+device.friendly_name+"</strong><div class='helper'>"+device.entity_id+" · "+device.state+"</div>";
        lbl.appendChild(input);
        lbl.appendChild(meta);
        els.deviceList.appendChild(lbl);
      });
    }
    function currentFormPayload(){
      const entityID=els.ttsEntity.value;
      const isCloud=isGoogleCloudEntity(entityID);
      const voice=isCloud?els.ttsVoice.value:"";
      const chirp=isCloud&&isChirpVoice(voice);
      return {
        message:els.broadcastMessage.value.trim(),
        target_entity_ids:selectedTargets(),
        tts_entity_id:entityID,
        language:els.ttsLanguage.value.trim(),
        voice:voice,
        input_mode:(!isCloud||chirp)?"text":els.ttsInputMode.value,
        speaking_rate:(!isCloud||chirp)?0:Number(els.ttsSpeakingRate.value||0),
        pitch:(!isCloud||chirp)?0:Number(els.ttsPitch.value||0),
        volume_gain_db:isCloud?Number(els.ttsVolumeGainDB.value||0):0,
        effects_profile_id:isCloud?els.ttsEffectsProfile.value.trim():""
      };
    }
    function hasUnsavedChanges(){
      if(!deployedSettings)return false;
      const cur=currentFormPayload();
      const depTargets=(deployedSettings.target_entity_ids||[]).slice().sort().join(",");
      const curTargets=(cur.target_entity_ids||[]).slice().sort().join(",");
      if(depTargets!==curTargets)return true;
      if((deployedSettings.tts_entity_id||"")!==(cur.tts_entity_id||""))return true;
      if((deployedSettings.voice||"")!==(cur.voice||""))return true;
      if(Number(deployedSettings.speaking_rate||0)!==Number(cur.speaking_rate||0))return true;
      if(Number(deployedSettings.pitch||0)!==Number(cur.pitch||0))return true;
      if(Number(deployedSettings.volume_gain_db||0)!==Number(cur.volume_gain_db||0))return true;
      if((deployedSettings.effects_profile_id||"")!==(cur.effects_profile_id||""))return true;
      return false;
    }
    function renderDeployedBanner(){
      if(!deployedSettings)return;
      const isCloud=isGoogleCloudEntity(deployedSettings.tts_entity_id);
      els.depEngine.textContent=isCloud?"Google Cloud TTS (預設)":"Google Translate (備援)";
      els.depVoice.textContent=isCloud?(deployedSettings.voice||"cmn-TW-Wavenet-A"):"Google Translate 台灣中文 (zh-tw)";
      if(isCloud){
        const parts=[];
        if(!isChirpVoice(deployedSettings.voice)){
          parts.push("語速 "+(deployedSettings.speaking_rate||1.0)+"x");
          parts.push("音高 "+(deployedSettings.pitch||0));
        }else{
          parts.push("Chirp 3: HD 自動語調");
        }
        if(deployedSettings.volume_gain_db)parts.push("音量 "+(deployedSettings.volume_gain_db>0?"+":"")+deployedSettings.volume_gain_db+"dB");
        els.depParams.textContent=parts.join(" · ")||"預設參數";
      }else{
        els.depParams.textContent="預設參數";
      }
      const names=(deployedSettings.target_entity_ids||[]).map((id)=>{
        const found=(broadcastOptions.media_players||[]).find((p)=>p.entity_id===id);
        return found?found.friendly_name:id;
      });
      els.depTargets.textContent=names.length?names.join(" + "):"未選擇";
    }
    function updateStudioState(){
      const payload=currentFormPayload();
      const hasMsg=payload.message.length>0;
      const hasTargets=payload.target_entity_ids.length>0;
      const hasEngine=Boolean(payload.tts_entity_id);

      els.broadcastButton.disabled=!(hasMsg&&hasTargets&&hasEngine);
      els.deployAutoButton.disabled=!(hasTargets&&hasEngine);

      const dirty=hasUnsavedChanges();
      if(dirty){
        els.diffBadge.textContent="● 有尚未部署的試聽變更";
        els.diffBadge.className="badge loop";
        els.resetToDeployedBtn.classList.remove("hidden");
      }else{
        els.diffBadge.textContent="✓ 與目前正式設定一致";
        els.diffBadge.className="badge active";
        els.resetToDeployedBtn.classList.add("hidden");
      }

      if(!hasTargets){
        els.broadcastSummary.textContent="請至少勾選一台 HomePod mini。";
      }else if(isGoogleCloudEntity(payload.tts_entity_id)){
        els.broadcastSummary.textContent="目前選擇 Google Cloud TTS · 聲線 "+(payload.voice||"cmn-TW-Wavenet-A")+" · 目標 "+payload.target_entity_ids.length+" 台 HomePod。";
      }else{
        els.broadcastSummary.textContent="目前選擇 Google Translate (zh-tw) · 目標 "+payload.target_entity_ids.length+" 台 HomePod。";
      }
    }
    function populateStudioFromDeployed(settings){
      deployedSettings=settings||{
        target_entity_ids:(broadcastOptions.media_players||[]).map((p)=>p.entity_id),
        tts_entity_id:broadcastOptions.default_tts_entity||"tts.google_cloud_tts_direct",
        language:"cmn-TW",
        voice:"cmn-TW-Wavenet-A",
        input_mode:"text",
        speaking_rate:0,
        pitch:0,
        volume_gain_db:0,
        effects_profile_id:""
      };
      renderTTSEntitySelect(deployedSettings.tts_entity_id);
      els.ttsLanguage.value=deployedSettings.language||(isGoogleCloudEntity(deployedSettings.tts_entity_id)?"cmn-TW":"zh-tw");
      selectedVoiceTier=inferVoiceTier(deployedSettings.voice||"cmn-TW-Wavenet-A");
      syncTierPillHighlight();
      els.ttsInputMode.value=deployedSettings.input_mode||"text";
      els.ttsSpeakingRate.value=deployedSettings.speaking_rate||"";
      els.ttsPitch.value=deployedSettings.pitch||"";
      els.ttsVolumeGainDB.value=deployedSettings.volume_gain_db||"";
      els.ttsEffectsProfile.value=deployedSettings.effects_profile_id||"";
      renderDeviceCheckboxes(deployedSettings.target_entity_ids);
      syncBroadcastFormUI(deployedSettings.voice||"cmn-TW-Wavenet-A");
      renderDeployedBanner();
      updateStudioState();
    }

    Array.from(els.tierPills.querySelectorAll(".tier-pill")).forEach((btn)=>{
      btn.addEventListener("click",()=>{
        selectedVoiceTier=btn.getAttribute("data-tier");
        syncTierPillHighlight();
        renderVoiceOptions("");
        syncBroadcastFormUI(els.ttsVoice.value);
      });
    });
    document.querySelectorAll("[data-preset]").forEach((btn)=>{
      btn.addEventListener("click",()=>{
        els.broadcastMessage.value=btn.getAttribute("data-preset");
        updateStudioState();
      });
    });
    els.resetToDeployedBtn.addEventListener("click",()=>{
      if(deployedSettings)populateStudioFromDeployed(deployedSettings);
      els.broadcastResult.textContent="已還原為目前正式通知設定。";
      els.broadcastResult.className="message";
    });
    els.ttsEntity.addEventListener("change",()=>syncBroadcastFormUI(""));
    els.ttsVoice.addEventListener("change",()=>syncBroadcastFormUI(els.ttsVoice.value));
    [els.ttsLanguage,els.ttsInputMode,els.ttsSpeakingRate,els.ttsPitch,els.ttsVolumeGainDB,els.ttsEffectsProfile,els.broadcastMessage].forEach((el)=>{
      el.addEventListener("input",updateStudioState);
      el.addEventListener("change",updateStudioState);
    });

    async function loadBroadcastStudio(){
      try{
        const [optRes,autoRes]=await Promise.all([
          fetch(broadcastOptionsURL,{cache:"no-store"}),
          fetch(broadcastAutoURL,{cache:"no-store"})
        ]);
        if(!optRes.ok)throw new Error("options HTTP "+optRes.status);
        broadcastOptions=await optRes.json();
        const autoData=autoRes.ok?await autoRes.json():null;
        populateStudioFromDeployed(autoData);
      }catch(e){
        els.broadcastResult.textContent="無法載入播報設定："+e.message;
        els.broadcastResult.className="message error";
      }
    }
    async function submitTestBroadcast(){
      const payload=currentFormPayload();
      els.broadcastButton.disabled=true;
      els.broadcastResult.textContent="🔊 正在產生語音並送往 HomePod 試聽...";
      els.broadcastResult.className="message";
      try{
        const r=await fetch(broadcastTestURL,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)});
        const text=await r.text();
        if(!r.ok)throw new Error(text||("HTTP "+r.status));
        els.broadcastResult.textContent="✓ 已送出試聽播報（"+(payload.voice||payload.tts_entity_id)+"）到："+payload.target_entity_ids.join(" / ") + "。若聲音滿意，請點旁邊「🚀 一鍵部署為正式通知設定」！";
        els.broadcastResult.className="message success";
      }catch(e){
        els.broadcastResult.textContent="❌ 試聽播報失敗："+e.message;
        els.broadcastResult.className="message error";
      }finally{
        updateStudioState();
      }
    }
    async function deployAutoBroadcastSettings(){
      const payload=currentFormPayload();
      els.deployAutoButton.disabled=true;
      els.broadcastResult.textContent="🚀 正在部署為正式通知播報設定...";
      els.broadcastResult.className="message";
      try{
        const r=await fetch(broadcastAutoURL,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)});
        const text=await r.text();
        if(!r.ok)throw new Error(text||("HTTP "+r.status));
        populateStudioFromDeployed(JSON.parse(text));
        els.broadcastResult.textContent="🎉 已成功一鍵部署為正式通知播報設定！之後垃圾車提醒將自動使用這組聲線與參數。";
        els.broadcastResult.className="message success";
      }catch(e){
        els.broadcastResult.textContent="❌ 部署正式設定失敗："+e.message;
        els.broadcastResult.className="message error";
      }finally{
        updateStudioState();
      }
    }

    els.broadcastButton.addEventListener("click",submitTestBroadcast);
    els.deployAutoButton.addEventListener("click",deployAutoBroadcastSettings);
    els.historyDate.addEventListener("change",(e)=>loadHistoryDay(e.target.value));

    async function refreshStatus(){
      try{
        const r=await fetch(statusURL,{cache:"no-store"});
        if(!r.ok)throw new Error("HTTP "+r.status);
        renderStatus(await r.json());
      }catch(e){
        els.statusMessage.textContent="狀態讀取失敗："+e.message;
      }
    }

    (async function init(){
      await refreshStatus();
      loadETAEvaluation();
      loadBroadcastStudio();
      try{
        await loadHistoryDatesAndLiveMap();
      }catch(e){
        els.historyMessage.textContent="歷史資料初始化失敗："+e.message;
        els.historyMessage.className="message error";
      }
      window.setInterval(async()=>{
        await refreshStatus();
        await loadETAEvaluation();
        if(selectedHistoryDate)await loadHistoryDay(selectedHistoryDate);
      },30000);
    })();
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
