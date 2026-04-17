# Changelog

## 0.6.9

- Normalize explicit TTS language codes like `zh-TW` to the lowercase hyphenated format Home Assistant actually accepts, such as `zh-tw`

## 0.6.8

- Make the historical sample table and status JSON panels scroll within fixed-height containers
- Add a dedicated UI section for automatic HomePod alert settings so the formal garbage-truck broadcast can choose its TTS engine, voice, language, and target players

## 0.6.7

- Make automatic HomePod alerts use the same direct TTS playback path as the test broadcast flow
- Fall back to the legacy webhook or service call path only when no playable media targets are available

## 0.6.6

- Make HomePod / Home Assistant speech alerts shorter and more natural while keeping Telegram alerts detailed

## 0.6.5

- Change the default collection days for 寶山鄉雙溪線 to Monday, Tuesday, Thursday, Friday, and Saturday

## 0.6.4

- Stop ending the collection run when the truck is merely near 有謙家園 or the target stop ETA reaches one minute
- Mark a run completed only after the route status reports the full trip has finished

## 0.6.3

- Fix garbage truck GPS collection to use Eupfin `Log_GISX` / `Log_GISY` when `GISX` / `GISY` are empty
- Match the live truck by `Car_Unicode` first, then fall back to `Route_ID`
- Preserve the route's `CarUnicode` and `Car_Number` in cached target state so Home Assistant can keep tracking the correct truck after refreshes

## 0.5.1

- Change the default collector check interval from `1m` to `20s`

## 0.5.0

- Add route-progress projection for ETA prediction, using Eupfin route points as a monotonic path axis
- Reduce crossing / overlap mispredictions by disambiguating with recent progress and movement heading
- Persist projected progress, segment, and lateral offset in SQLite and auto-migrate existing databases
- Add fake-data tests for crossings, lateral offset tolerance, route-shape fallback, and historical prediction matching

## 0.4.3

- Improve Home Assistant TTS fallback handling when the preferred engine cannot generate playable audio

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

## 0.4.2

- Add Gemini voice selection to the App UI test panel
- Default Gemini test playback to `achernar`, which is more reliable for Chinese on the bedroom HomePod
