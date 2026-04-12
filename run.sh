#!/usr/bin/with-contenv bashio

set -euo pipefail

bashio::log.info "Starting Garbage ETA Predictor"

export TELEGRAM_BOT_TOKEN="$(bashio::config 'telegram_bot_token')"
export TELEGRAM_CHAT_ID="$(bashio::config 'telegram_chat_id')"

export HA_NOTIFY_MODE="$(bashio::config 'ha_notify_mode')"
export HA_TTS_TARGET="$(bashio::config 'ha_tts_target')"
export HA_BASE_URL="http://supervisor/core"
export HA_TOKEN="${SUPERVISOR_TOKEN}"

export TARGET_CUST_ID="$(bashio::config 'target_cust_id')"
export TARGET_ROUTE_ID="$(bashio::config 'target_route_id')"
export TARGET_POINT_SEQ="$(bashio::config 'target_point_seq')"
export TARGET_POINT_NAME="$(bashio::config 'target_point_name')"
export TARGET_DAYS="$(bashio::config 'target_days')"
export ALERT_OFFSETS="$(bashio::config 'alert_offsets')"
export COLLECTION_START="$(bashio::config 'collection_start')"
export COLLECTION_END="$(bashio::config 'collection_end')"
export HISTORY_WEEKS="$(bashio::config 'history_weeks')"
export ARRIVAL_RADIUS_METERS="$(bashio::config 'arrival_radius_meters')"
export MATCH_RADIUS_METERS="$(bashio::config 'match_radius_meters')"
export MIN_HISTORY_RUNS="$(bashio::config 'min_history_runs')"
export CHECK_INTERVAL="$(bashio::config 'check_interval')"
export SEND_TEST_MESSAGE_ON_START="$(bashio::config 'send_test_message_on_start')"
export PORT="$(bashio::config 'status_port')"

target_time=""
if bashio::config.has_value 'target_time'; then
	target_time="$(bashio::config 'target_time')"
fi
export TARGET_TIME="${target_time}"

export STATE_FILE="/data/state.json"
export DATABASE_FILE="/data/history.db"

mkdir -p /data

exec /app/server
