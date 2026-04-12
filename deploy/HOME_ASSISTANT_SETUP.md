# Home Assistant / Raspberry Pi Beginner Setup

## Which path should you use?

- If you already run **Home Assistant OS**, install this repo as a **Home Assistant App** instead of following this document.
- If you run **Raspberry Pi OS + Docker** and want Home Assistant plus this service side-by-side, this document is still the right path.

For Home Assistant OS, read the repo root `DOCS.md`.

## Recommended path

For this repo, the simplest deployment path is:

1. Raspberry Pi OS
2. Docker Engine + `docker compose`
3. Home Assistant Container
4. `garbage-reminder` as a second container on the same Pi

Why this path:

- Home Assistant officially recommends Home Assistant OS for most users.
- But this project is currently a standalone container, not a Home Assistant add-on.
- So if you want both Home Assistant and this service on the same Pi with the least friction, `Home Assistant Container + garbage-reminder` is the most direct setup.

## Step 1: Prepare the Raspberry Pi

On the Pi, make sure you already have:

- Raspberry Pi OS installed
- Docker Engine installed
- `docker compose` working

If Home Assistant is not installed yet, use the official Home Assistant Container path on Raspberry Pi:

- Official docs: <https://www.home-assistant.io/installation/raspberrypi-other>

## Step 2: Put this repo on the Pi

```bash
git clone <your-repo-url>
cd telegram-garbage-reminder
cp .env.example .env
```

Edit `.env` and fill at least:

- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_CHAT_ID`
- `HA_BASE_URL`
- `HA_TOKEN`
- `HA_NOTIFY_MODE`
- `HA_TTS_TARGET`

Recommended values when Home Assistant runs on the same Pi:

```env
HA_BASE_URL=http://127.0.0.1:8123
HA_NOTIFY_MODE=webhook
HA_TTS_TARGET=garbage_truck_eta
```

## Step 3: Start Home Assistant and garbage-reminder

If you want both services managed in one compose stack:

```bash
cd deploy
docker compose -f compose.rpi-ha-example.yaml up -d --build
```

If you already have Home Assistant running elsewhere, only run this app as a container and point `HA_BASE_URL` to your existing HA instance.

## Step 4: In Home Assistant, make HomePod Mini usable

### 4.1 Add the HomePod Mini device

If the HomePod Mini is not already visible in HA:

1. Go to `Settings > Devices & Services`
2. Add or open the `Apple TV` integration

Official docs:

- Apple TV integration: <https://www.home-assistant.io/integrations/apple_tv/>

What you want to end up with:

- A `media_player.*` entity for the HomePod Mini

### 4.2 Add a TTS provider

Recommended beginner option:

1. Go to `Settings > Devices & Services`
2. Add `Google Translate text-to-speech`

Official docs:

- TTS overview: <https://www.home-assistant.io/integrations/tts>
- Google Translate TTS: <https://www.home-assistant.io/integrations/google_translate>

What you want to end up with:

- A `tts.*` entity, for example `tts.google_translate_en_com`

## Step 5: Connect HA broadcast to this app

### Recommended: webhook mode

This repo already knows how to POST to:

- `POST /api/webhook/<HA_TTS_TARGET>`

If your `.env` contains:

```env
HA_NOTIFY_MODE=webhook
HA_TTS_TARGET=garbage_truck_eta
```

then in Home Assistant you can copy the YAML from:

- `deploy/home_assistant/automation_webhook.yaml`

Before saving it, replace:

- `tts.google_translate_en_com`
- `media_player.replace_with_your_homepod`

### Optional: service_call mode

If later you prefer `service_call` instead of webhook:

```env
HA_NOTIFY_MODE=service_call
HA_TTS_TARGET=script.homepod_broadcast
```

Then copy:

- `deploy/home_assistant/script_service_call.yaml`

and again replace the TTS entity and HomePod media player entity.

## Step 6: Optional status sensor in Home Assistant

This service exposes:

- `GET http://<pi-ip>:8080/status`

If you want ETA data inside HA, copy:

- `deploy/home_assistant/rest_status_sensor.yaml`

Official docs:

- REST sensor: <https://www.home-assistant.io/integrations/sensor.rest/>

## Step 7: Test the whole chain

### Test Home Assistant webhook manually

From the Pi:

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"message":"測試廣播，垃圾車快到了","source":"garbage-tracing"}' \
  http://127.0.0.1:8123/api/webhook/garbage_truck_eta
```

If this works, HomePod Mini should speak.

### Test service status

```bash
curl http://127.0.0.1:8080/status
```

You should see JSON with fields like:

- `active`
- `service_date`
- `prediction`
- `notified_offsets`

## Important notes

- Webhook mode is easier for beginners than `service_call`.
- `tts.speak` works against a `tts.*` entity and a `media_player.*` target.
- If TTS does not play, first check Home Assistant’s local URL under `Settings > System > Network`, because the official TTS docs note that local URL configuration affects media playback.
- If you already run Home Assistant OS instead of Home Assistant Container, this standalone-container design is not the easiest fit. In that case, tell me your current HA installation type and I’ll adapt the deployment path for HA OS specifically.
