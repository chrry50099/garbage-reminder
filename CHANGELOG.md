# Changelog

## 0.2.0

- Add Home Assistant OS App packaging
- Use Supervisor internal API defaults when available
- Keep standalone Docker deployment compatibility

## 0.2.1

- Avoid hard-coded host port conflicts in Home Assistant OS
- Make app startup retry instead of exiting on initial validation failure

## 0.2.2

- Fix Home Assistant OS builds producing the wrong CPU architecture binary

## 0.3.0

- Add a built-in Home Assistant Ingress UI and sidebar entry
- Show ETA, GPS, notification status, and raw `/status` JSON in the App page

## 0.4.0

- Add a HomePod Mini test broadcast panel to the App UI
- Let the App discover media players and TTS entities from Home Assistant
- Add backend endpoints for test message sending from the sidebar page

## 0.4.1

- Prefer Google Gemini TTS as the default cloud TTS choice in the App UI
- Omit the `language` field for Gemini TTS so Chinese playback works correctly
