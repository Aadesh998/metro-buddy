# Android build and go-live

Production API: https://api-production-ebaf.up.railway.app (Railway project `metro-buddy`, service `api`, with Postgres and Redis).
Admin console: https://api-production-ebaf.up.railway.app/admin

## 1. Make the server usable for real sign-ups

Set these on the `api` service (`railway variable set KEY=value --service api`, or the Railway dashboard). Each change redeploys.

| Variable | Why | Without it |
|---|---|---|
| `MSG91_AUTH_KEY`, `MSG91_OTP_TEMPLATE_ID` | OTP login SMS (DLT-approved template) | Nobody can log in: OTP request returns 503 |
| `MSG91_SOS_FLOW_ID` | SOS texts to trusted contacts | SOS still alerts the safety team, but no SMS |
| `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_FROM` | SMS fallback | Optional |
| `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `R2_BUCKET` | Image storage | Images live in memory and vanish on redeploy |
| `NSFW_ENDPOINT` | Automatic image scan | Every image waits for a moderator |
| `ANTHROPIC_API_KEY` | AI PII second pass, triage, icebreakers, topic ideas | Those features fall back to non-AI behaviour |
| `EXPO_ACCESS_TOKEN` | Push notifications | Push still works for most cases; set it for reliability |
| `LIVEKIT_URL`, `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET` | Vent Studio audio (LiveKit Cloud) | Studio rooms can't connect |
| `LIVENESS_PROVIDER=rekognition`, `AWS_*` | Selfie verification | Verification endpoint returns 503 |
| `PLAY_INTEGRITY_CREDENTIALS_JSON`, `APPLE_APP_ID` | Device integrity | No attestation; nobody gets restricted |

In LiveKit Cloud, add the webhook `https://<api domain>/v1/livekit/webhook`.

For a custom domain: `railway domain api.metrobuddy.in --service api`, add the CNAME it prints at your DNS provider, then set `PUBLIC_BASE_URL=https://api.metrobuddy.in` and update `EXPO_PUBLIC_API_URL` in `apps/mobile/eas.json`.

More admins: create a moderator from **Team** in the admin console (the first admin was created with `server/cmd/admin`).

## 2. Build the Android app

One-time setup (needs a free Expo account):

```bash
npm install -g eas-cli
eas login
cd apps/mobile
eas init            # creates the EAS project and writes its id into app.json
```

Builds run on Expo's servers; no Android Studio needed.

**Test APK (install directly on phones):**
```bash
eas build -p android --profile preview
```
When it finishes, EAS prints a link and QR code. Open it on an Android phone, download the APK and install (allow "install unknown apps"). This build talks to the Railway server.

**Development build (for live debugging with your laptop):**
```bash
eas build -p android --profile development   # install once on the phone
npx expo start --dev-client                    # then run this; the app loads code from your laptop
```
Expo Go can't be used: LiveKit audio and other native modules need this development build. The `development` profile points at `http://10.0.2.2:8080` (a local server seen from the Android emulator); for a real phone change it to your laptop's LAN IP or the Railway URL.

**Store build (Play Store AAB):**
```bash
eas build -p android --profile production
```

## 3. Things to test on a real phone before launch

- Sign up with a real number (needs MSG91), age gate, consents, persona, home metro, vibes.
- Push notifications: DM request, match, episode reminder.
- Join a bowl, post with a photo, comment, report, block.
- Two phones: DM request → chat, reveal ladder, disappearing messages, photo in chat.
- Travel Now on two phones on the same line → match → trip → SOS → share trip link.
- Studio (needs LiveKit Cloud): host goes live, second phone listens with the screen locked, calls in, host stages and cuts the mic.
- Switch to हिन्दी on the Me tab.

## 4. Publish on Google Play

1. Create a Google Play developer account (one-time $25) at https://play.google.com/console.
2. Create the app `in.metrobuddy.app`. Fill in **App content** using `docs/launch/privacy-labels.md` (data safety), `docs/launch/ugc-compliance.md` (UGC policy, 18+ target audience, content rating questionnaire), and `docs/launch/store-listing.md` (title, descriptions, screenshots).
3. In Play Console → Setup → API access, create a service account with release permissions, download its JSON key, and save it as `apps/mobile/play-service-account.json` (never commit it; add `"serviceAccountKeyPath": "./play-service-account.json"` under `submit.production.android` in `eas.json`).
4. Upload the first build: `eas submit -p android --profile production` (goes to the **internal** track as configured).
5. Promote internal → closed testing → production in Play Console. New personal developer accounts must run a closed test with at least 12 testers for 14 days before production access.
6. Use the same AAB's Play Integrity settings (Play Console → App integrity) and put its service account JSON into `PLAY_INTEGRITY_CREDENTIALS_JSON` on Railway.

## 5. Shipping updates after launch

- Server: `railway up --service api` from the repo root (or connect the GitHub repo in Railway for deploy-on-push).
- JS-only app changes: `eas update --channel production --message "what changed"` — users get it on next launch, no store review.
- Native changes (new native package, app.json plugin, permissions): new `eas build` + `eas submit`.
- Bump `version` in `app.json` for store releases; build numbers auto-increment.

## 6. Operations

- Logs: `railway logs --service api`
- Health: `curl https://<api domain>/healthz`
- Database console: `railway connect Postgres`
- Back up before big changes: Railway dashboard → Postgres → Backups.
- `railway.toml` (Config as Code) keeps working until 2026-12-01; migrate with `railway config migrate` before then.
