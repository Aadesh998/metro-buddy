# Free and open-source services

What Metro Buddy uses instead of paid vendors, and where each runs.

| Need | Open-source choice | Where it runs | Cost |
|---|---|---|---|
| Image storage | **RustFS** (Apache-2.0, S3-compatible; drop-in for MinIO) | Railway service `storage` with a volume | Railway usage only |
| Image safety scan | **services/nsfw**: Yahoo Open NSFW model via `opennsfw2` (MIT) | Railway service `nsfw`, private network only | ~400 MB RAM |
| Live audio | **LiveKit server** (Apache-2.0) | A small VPS (needs UDP; Railway has none) | VPS cost, or LiveKit Cloud free tier |
| SMS (OTP, SOS) | **SMS Gateway for Android** (Apache-2.0, capcom6/android-sms-gateway) | An Android phone with a SIM | The SIM's SMS plan |
| Push | Expo push service | Expo | Free |
| AI features | none needed | — | Features fall back to non-AI behaviour without a key |
| Selfie liveness, device integrity | no reliable open-source equivalent | — | Leave off until needed |

MinIO itself no longer publishes free container images, so RustFS is used; Garage and SeaweedFS also work, since the server only needs the S3 API (`S3_*` variables).

## Storage (RustFS)

Set up in both environments: services `storage` (staging) and `prod-storage` (production), each with its own volume and keys.

- Service `storage` from image `rustfs/rustfs:latest`, volume mounted at `/data`, public domain on port 9000.
- Variables on `storage`: `RUSTFS_ACCESS_KEY`, `RUSTFS_SECRET_KEY` (random).
- Variables on `api`: `S3_ENDPOINT=https://<storage domain>`, `S3_ACCESS_KEY=${{storage.RUSTFS_ACCESS_KEY}}`, `S3_SECRET_KEY=${{storage.RUSTFS_SECRET_KEY}}`, `S3_BUCKET=metro-buddy`.

The API creates the bucket on boot. The bucket stays private: phones only get short-lived signed URLs.

## Image scanner (services/nsfw)

`services/nsfw` is a small Flask service: `POST /score` with image bytes returns `{"nsfw": 0..1}`. The API blocks above 0.85, sends 0.4–0.85 to moderators, and clears below 0.4.

- Services `nsfw` (staging) and `prod-nsfw` (production), deployed with `railway up services/nsfw --path-as-root --service nsfw`. No public domain; the API reaches it at `http://${{nsfw.RAILWAY_PRIVATE_DOMAIN}}:8000/score` with `NSFW_TOKEN=${{nsfw.NSFW_TOKEN}}`.
- Images are scored in memory and never stored.

## SMS (SMS Gateway for Android)

Indian carriers limit SMS per SIM per day (roughly 100), and sending app OTPs from a personal SIM isn't DLT-compliant at commercial scale. Good for testing and early users; move to MSG91 (or another DLT-registered provider) before growing.

1. Install **SMS Gateway for Android** from https://sms-gate.app (or F-Droid / GitHub releases) on a phone with an active SIM. Keep it charged and online.
2. In the app, turn on **Cloud server**. It shows a username and password.
3. On Railway (`api` service):
   ```
   SMS_PROVIDER=gateway
   SMS_GATEWAY_USER=<username from the app>
   SMS_GATEWAY_PASSWORD=<password from the app>
   ```
   `SMS_GATEWAY_URL` defaults to the public relay `https://api.sms-gate.app/3rdparty/v1/messages`. For full self-hosting, run the project's private server and point the URL at it.
4. Remove `OTP_DEV_CODE` once real SMS works.

## Live audio (self-hosted LiveKit)

WebRTC audio needs UDP ports, which Railway doesn't offer, so LiveKit runs on a VPS. Any 1–2 vCPU / 2 GB Linux VM works (Oracle Cloud's Always Free ARM VM, Hetzner, DigitalOcean, etc.). LiveKit Cloud's free tier is the no-server alternative.

1. Point a DNS name at the VM, e.g. `livekit.metrobuddy.in`.
2. Open firewall ports: TCP 80, 443, 7881; UDP 50000–60000; UDP 3478.
3. On the VM (Docker installed), create `livekit.yaml`:
   ```yaml
   port: 7880
   rtc:
     tcp_port: 7881
     port_range_start: 50000
     port_range_end: 60000
     use_external_ip: true
   keys:
     <API_KEY>: <API_SECRET>          # openssl rand -hex 16 / openssl rand -hex 32
   webhook:
     api_key: <API_KEY>
     urls: ["https://<api domain>/v1/livekit/webhook"]
   turn:
     enabled: true
     udp_port: 3478
   ```
   and `Caddyfile` for TLS:
   ```
   livekit.metrobuddy.in {
     reverse_proxy localhost:7880
   }
   ```
4. Run:
   ```bash
   docker run -d --name livekit --network host -v $PWD/livekit.yaml:/etc/livekit.yaml livekit/livekit-server --config /etc/livekit.yaml
   docker run -d --name caddy --network host -v $PWD/Caddyfile:/etc/caddy/Caddyfile -v caddy_data:/data caddy:2
   ```
5. On Railway (`api`): `LIVEKIT_URL=wss://livekit.metrobuddy.in`, `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET` (the pair from step 3).

Recording is never enabled: the server stops any egress it's told about.

## Test APK

`MetroBuddy-staging.apk` is built locally (no Expo account needed):

```bash
cd apps/mobile && npx expo prebuild -p android --no-install
cd android && JAVA_HOME=<jdk 17> ANDROID_HOME=~/Android/Sdk \
  EXPO_PUBLIC_API_URL=https://api-staging-b0d3.up.railway.app NODE_ENV=production \
  ./gradlew assembleRelease -PreactNativeArchitectures=arm64-v8a
```

It is signed with the debug key, so it's for testing only: Play Store builds (EAS) use a different key, and phones must uninstall the test APK before installing a store build.
