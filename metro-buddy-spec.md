# Metro Buddy — build prompt, tech stack & functional spec

An anonymous social app for Indian metro commuters, built as a React Native
(Expo) app for iOS and Android. It has a Go API, Postgres, Redis and live
audio. Hand this file to the coding agent as the source of truth.

Companion files (same folder):
- `metro-buddy-ontology.md`: the full data ontology (entities, fields, enums, relationships, privacy classes, retention).
- `metro-buddy-plan.xlsx`: Features, Tasks, Phase Summary, Entities, Fields and Enums, one tab each.
- `tasks.csv`: the live task tracker (status and testing).

---

## 0. Guardrails for the coding agent (read first, apply always)

**Token & output discipline**
- **Minimal comments.** Write no comments unless the *why* is non-obvious: a security decision, a workaround or a tricky invariant. Never write comments that restate the code. Never write file-header banners, JSDoc/GoDoc on obvious functions, or `// TODO` filler.
- No narration in code ("Step 1: …", "Now we …"). No commented-out code.
- Don't reprint unchanged files or large unchanged blocks. Use targeted edits and show diffs, not whole files.
- Don't generate READMEs, docs, changelogs or example files unless asked.
- Don't add speculative abstractions, config knobs, or "future-proof" layers. Build exactly the task in scope.
- Keep explanations after a task to a few lines: what changed, what to run, and anything that's blocked.
- Reuse existing helpers and components before creating new ones. Grep first.
- One task = one small, reviewable change. Work from `tasks.csv`:
  - Pick the next `Todo` task whose `Depends on` tasks are all `Done`, in Task ID order within the current phase.
  - Set `Status` to `In progress` with a `Started` date (`YYYY-MM-DD`) when you begin, and to `Review` when the code is ready. Only the user sets `Done`.
  - Keep `Dev notes` to one line. Don't add or reorder rows without asking.
  - **Never edit the test columns** (`Test status`, `Tested on`, `Tested by`, `Test date`, `Bugs / test notes`). The user fills them in manually.
  - If a task's `Test status` is `Fail`, fix it before starting new work, then set `Status` back to `Review`.

**Product & safety guardrails (never violate)**
- A user's real identity (phone number, real name, photo, exact location) is **never** sent to another user's client unless both people explicitly consent through the Reveal flow (F08).
- Social tables reference `persona_id`, **never** `user_id`. Only the identity, safety and moderation domains may join Persona → User.
- All user-generated text goes through the PII masking pipeline (F24) **before** it is stored or fanned out.
- Exact GPS coordinates are never stored. Location is resolved to the nearest station on the device or at the API edge, and then the coordinates are discarded.
- Nobody can be messaged without consent: a DM starts as a request, and a travel match unlocks chat only after both sides accept.
- AI recommends; humans decide. AI never bans, deletes or reveals anything on its own. It only flags and suggests.
- Self-harm signals show helplines (Tele-MANAS 14416, emergency 112). They never trigger an auto-ban or shadow action.
- 18+ only. Report, block and mute must be reachable from every piece of UGC and every profile (App Store guideline 1.2 and Play UGC policy).

---

## 1. Idea

Millions of people in Delhi, Mumbai, Bengaluru, Hyderabad, Chennai, Pune and Kolkata ride the metro every day. They ride solo or packed in among strangers, and nobody talks. Many of them are frustrated with office politics, college pressure, the daily grind, cancelled plans and friends who ditch them. **Metro Buddy** gives them a safe, anonymous place to:

1. **Vent and discuss** in Fishbowl-style communities called **Bowls** (per metro line, station, office hub, college or topic). Posts are Reddit-style and every author is anonymous.
2. **Chat** 1:1 or in groups, with sensitive info masked automatically.
3. **Join group discussions** in time-boxed live text rooms around a topic ("Monday Office Rant", "Plans Cancelled Again").
4. **Call in to a Vent Studio.** A host (the "RJ") runs a live audio show and listeners raise a hand to come on air and let it out, anonymously.
5. **Find a Travel Buddy.** Solo commuters are matched with people on the same route at the same time with a matching vibe. If both accept, they meet at the station and can become a regular **Commute Crew**.

**What it deliberately is NOT:** not a dating app (no swiping on photos, no attractiveness ranking), not a public-identity network, not a location tracker, and not a ride-share/cab app.

---

## 2. Tech stack

| Layer | Choice | Why |
|---|---|---|
| Mobile | **React Native via Expo** (latest stable SDK), TypeScript, **Expo Router** | One codebase for iOS and Android, OTA updates via EAS Update, managed native modules |
| Mobile styling | **NativeWind** (Tailwind for RN) + own small primitives | Same Tailwind mental model as the web template |
| Icons | `lucide-react-native` | Same as the template |
| Server state | **TanStack Query** + a thin typed API client | Caching/retry/pagination for mobile networks. No Redux. Local UI state stays in components; use Zustand only if 3+ screens share mutable client state |
| Lists | `@shopify/flash-list` | Chat and feeds need virtualization |
| Secure storage | `expo-secure-store` (Keychain/Keystore) | Refresh token and device key. Never AsyncStorage for secrets |
| Push | `expo-notifications` (APNs + FCM via Expo push) | |
| Live audio | **LiveKit** (LiveKit Cloud to start, self-host later) + `@livekit/react-native` | Low-latency WebRTC rooms with host/speaker/listener roles managed from the server |
| Backend | **Go + Echo v4** | Carried over from the template |
| ORM / DB | **GORM + Postgres** (Railway), SQLite for unit tests | Carried over from the template |
| Realtime | **WebSocket** gateway in the Go binary (`coder/websocket`) + **Redis pub/sub** for fan-out across instances | Chat and presence need push. Redis is justified because more than one API instance must share fan-out, presence, rate limits and OTP throttles |
| Object storage | Cloudflare R2 (S3 API), pre-signed uploads | Images in posts and chat |
| SMS OTP | MSG91 (India DLT-compliant), Twilio as fallback | Indian phone-first login |
| AI | Anthropic API: **Claude Haiku 5.5** for moderation/PII second pass, **Claude Sonnet 5.5** for icebreakers and host topic ideas | Cheap, fast classifier plus a better writer where quality matters |
| Admin console | React + Vite SPA embedded in the Go binary under `/admin` | Reuses the web template pattern for moderators |
| Public site | Go `html/template` + htmx: landing page, privacy policy, terms, grievance officer page | Needed for store listings and the IT Rules |
| Hosting | Railway: one API service + Postgres + Redis | |
| Builds | EAS Build + EAS Submit | Store builds without local Xcode/Gradle hassle |
| Metro data | Seeded from public GTFS feeds (e.g. Delhi Open Transit Data, DMRC/BMRCL/MMRDA published station lists) | Station graph for route-overlap matching |

**Deliberately not used (v1):** no microservices, no Kubernetes, no GraphQL, no Firebase as the primary DB, no map SDK (a station picker is enough), no background location tracking, no recording of audio shows.

---

## 3. Architectural principles

1. **Two-layer identity.** `User` (private: phone hash, consents, devices) is separate from `Persona` (public: alias, avatar, vibe). Every social object belongs to a Persona. Only the identity, safety and moderation services can cross that boundary, and every crossing is audit-logged.
2. **Mask before persist.** UGC goes through the pipeline `client submit → API → PII masker (regex + rules) → [async LLM second pass] → store masked → fan out`. The masked text is canonical. The raw text is stored only in an encrypted `pii_vault` with a 90-day TTL for legal and law-enforcement requests, and access to it requires the `trust_safety_lead` role and is audit-logged.
3. **Consent-gated contact.** Opening a conversation needs an accepted DM request, an accepted match, membership in a group, or the room being public. This is checked on the server for every send.
4. **Authorization at the query layer.** Each read is scoped through helpers (`scopeConversationMember`, `scopeBowlVisible`, `excludeBlocked`). Blocks are bidirectional and apply everywhere (feeds, search, matching, rooms).
5. **Database is truth, instances are stateless.** Postgres holds durable state. Redis holds ephemeral state only (presence, typing, rate limits, OTP throttles, live-intent cache) and can be lost without data loss.
6. **Location minimization.** We store a station, never coordinates. "Travel Now" intents expire automatically after 45 min.
7. **Safety beats growth.** New accounts get rate limits (DMs/day, posts/hour) that relax as trust score rises. Never optimize engagement in ways that weaken these.
8. **Small, reviewable increments**, following the task order in the plan workbook.

---

## 4. Identity, PII masking & safety design

**Login:** phone OTP (6 digits, 5 min TTL, max 5 attempts, per-number and per-IP throttling in Redis). The phone number is stored as an `HMAC-SHA256(pepper, E.164)` for lookup plus an AES-GCM ciphertext for sending SMS. It is never returned by any API.

**Session:** a 15-minute JWT access token plus a 30-day opaque refresh token, rotated on every use, with reuse detection that revokes the whole token family. Mobile stores the refresh token in `expo-secure-store` and sends `Authorization: Bearer`, since there are no cookies on native.

**Persona:** an auto-generated alias (`adjective + noun + 2 digits`, e.g. "Chai Comet 27") and a generated avatar (seeded shapes/colors, no photos in v1). The user can re-roll on signup and change the alias once every 30 days. Old aliases are kept in `persona_alias_history` for moderation.

**PII masker (F24): detect and replace with typed tokens**
| Pattern | Example | Replaced with |
|---|---|---|
| Indian mobile (+91 / 0 / bare 10-digit starting 6–9, with spaces/dashes) | 98765 43210 | `[phone hidden]` |
| Email | a@b.com | `[email hidden]` |
| UPI ID | name@okicici | `[upi hidden]` |
| Aadhaar (12 digits, Verhoeff check) | 1234 5678 9012 | `[id hidden]` |
| PAN | ABCDE1234F | `[id hidden]` |
| Social handles/links (instagram, snap, telegram, wa.me, t.me) | @riya_23, insta: riya | `[handle hidden]` |
| URLs not on the allow-list | | `[link hidden]` |
| Spelled-out numbers ("nine eight seven…") and digits split across messages | | caught by the LLM second pass, which updates `masked_text` and emits a `message.updated` event |

Once both people have completed the Reveal flow (F08) at the `contact` level, phone and handle masking is lifted **for that conversation only**.

**Reveal ladder (F08):** `none → first_name → photo → contact`. Each step needs *both* people to tap "reveal". Either person can revoke for future messages.

**Safety features:** report/block/mute everywhere; trust score; SOS (sends trusted contacts an SMS with a live trip link showing station-level progress); a "share my trip" button for every travel match; same-gender-only and verified-only matching filters; meet points restricted to inside station premises (platform + coach).

**Compliance:** DPDP Act 2023 (itemised consent, data export, deletion within 30 days, Data Protection Officer contact), IT Rules 2021 (grievance officer, 24 h acknowledgement and 15-day resolution, 24 h takedown for non-consensual intimate content), and Apple/Google UGC policies.

---

## 5. Security & performance (carried over from the template, adapted)

- **IDs:** UUIDv7 primary keys everywhere (client-generatable for optimistic chat sends and idempotency, not enumerable). No Sqids layer.
- **DTOs are separate from GORM models.** No handler returns a model. Persona DTOs never carry `user_id`.
- **Validation:** `go-playground/validator` on every request DTO. Re-derive `user_id`/`persona_id` from the token and never trust a body field for them.
- **Rate limits** (Redis, sliding window): OTP, login, DM requests, posts, comments, reports, match requests, call-in requests. Tiers are keyed by trust level.
- **WebSocket auth:** a short-lived WS ticket (30 s, single use) obtained over HTTPS, so JWTs never go in URLs.
- **Media:** pre-signed R2 PUT with a size cap (5 MB) and MIME allow-list; EXIF stripped server-side; NSFW scan before the image becomes visible.
- **Secrets:** Railway env vars only. A logging denylist covers phone, OTP, tokens, raw UGC and coordinates.
- **Mobile:** certificate pinning is optional in v2. Play Integrity / App Attest check at signup (v1.1). No secrets in the app bundle.
- **OWASP Top 10:** same checklist as the template. Most relevant here: A01 (IDOR on conversations, bowls and matches, so check membership on every request), A03 (sanitize Markdown in posts), A07 (OTP brute force) and A10 (link previews: block private IP ranges and re-check after redirects).
- **Performance:** cursor pagination everywhere (`created_at, id`). Index every FK and filter column. Use `Preload` to avoid N+1 queries in feeds. Feed "hot" score is precomputed every 5 min. Set connection pool limits explicitly.

---

## 6. Core entities (summary, full detail in `metro-buddy-ontology.md`)

```
User 1─1 Persona ─┬─< BowlMember >── Bowl ──< Post ──< Comment
                  ├─< ConversationMember >── Conversation(dm|group|crew|discussion) ──< Message
                  ├─< CommutePlan ──> Station(from/to)        Station >── LineStation ──< MetroLine ── City
                  ├─< TravelIntent ──> Match <── TravelIntent
                  ├─< Trip >── TripParticipant / TripFeedback
                  ├─< Show (host) ──< ShowEpisode ──< CallInRequest
                  └─< Report / Block / Mute
```

---

## 7. Features (build order)

Feature IDs match the `Features` tab. MVP = phases P0–P3.

**P0 Foundation:** repo setup (Expo app + Go API + admin), CI, auth (F01), persona (F02), age gate and consents (F05), metro data seed (city/line/station graph), home metro setup (F04), vibe profile (F03), report/block/mute plumbing (F50).

**P1 Bowls (Fishbowl/Reddit):** bowl discovery/join (F10), posts with text/image/poll (F11), threaded comments and votes (F12), feed ranking (F13), daily prompts (F15), PII masking on all UGC (F24), moderation queue v1 (F51).

**P2 Chat & discussion:** DM requests (F20), group chats (F21), realtime WS gateway with typing, receipts and offline sync (F23), live discussion rooms (F22), push notifications and inbox (F60, F61).

**P3 Travel Buddy:** commute plans (F40), Travel Now (F41), match engine (F42), request/accept → chat (F43), meet point (F44), trip lifecycle and feedback (F45), matching preferences (F47), SOS and share trip (F53). **MVP launch here, one city (Delhi NCR) first.**

**P4 Vent Studio (RJ):** host profiles and application (F30), scheduled shows (F31), live audio rooms (F32), call-in queue (F33), reactions and text questions (F35), on-air safety controls (F36).

**P5 Trust & growth:** Commute Crews (F46), Reveal ladder (F08), trust score (F52), user-created bowls (F14), AI moderation assist (F80), icebreakers (F81), host topic ideas (F82), Hindi i18n (F74), disappearing messages (F26).

**P6 Later:** selfie verification badge (F07), voice masking for callers (F34), device integrity (F06), more cities.

**Match engine (F42) scoring:**
`score = 0.45·route_overlap + 0.20·time_fit + 0.30·vibe_jaccard + 0.05·trust_bonus`
- `route_overlap` = shared stations ÷ shorter trip's stations, computed by BFS on the station graph. A candidate needs ≥ 0.4.
- `time_fit` = `1 − |Δdeparture| / 20 min`, floored at 0. A candidate needs |Δ| ≤ 20 min.
- Hard filters before scoring: blocked either way, gender preference, verified-only, age band, active intent window.
- Return the top 10. Show the "why" chips ("Same line · 5 min apart · both love indie music"). Never show exact home stations, only the overlapping segment.

**Vent Studio roles:** `host`, `co_host`, `on_air` (one caller at a time, max 8 min, host can extend), `listener`. Listeners send a call-in request with a ≤ 80-char topic, the host picks from the queue, the server issues a LiveKit token with publish rights, and that token is revoked when the caller drops. Recording is off and there's no replay in v1.

---

## 8. API surface (REST + WS)

```
POST /v1/auth/otp/request   POST /v1/auth/otp/verify   POST /v1/auth/refresh   POST /v1/auth/logout
GET/PATCH /v1/me  /v1/me/persona  /v1/me/vibes  /v1/me/settings  /v1/me/consents
POST /v1/me/export   DELETE /v1/me
GET /v1/cities  /v1/cities/:id/lines  /v1/stations?q=
GET /v1/bowls?suggested=1   POST /v1/bowls/:id/join   GET /v1/bowls/:id/posts?cursor=
POST /v1/posts  GET /v1/posts/:id  POST /v1/posts/:id/comments  POST /v1/votes
POST /v1/dm-requests  POST /v1/dm-requests/:id/accept
GET /v1/conversations  GET /v1/conversations/:id/messages?cursor=  POST /v1/conversations/:id/messages
GET /v1/rooms/live  POST /v1/rooms/:id/join
POST /v1/commute-plans  POST /v1/travel-intents  GET /v1/matches  POST /v1/matches/:id/accept
POST /v1/trips/:id/checkin  POST /v1/trips/:id/feedback  POST /v1/sos
GET /v1/shows  POST /v1/episodes/:id/join  POST /v1/episodes/:id/call-in  POST /v1/episodes/:id/stage (host)
POST /v1/reports  POST /v1/blocks  POST /v1/mutes
POST /v1/reveal/:conversationId/:level
POST /v1/ws/ticket   GET /ws?ticket=   (events: message.new, message.updated, typing, receipt, match.new, room.presence, episode.stage)
/admin/api/*  moderation queue, actions, user lookup (audited), metrics
```

---

## 9. Repo layout

```
apps/mobile/                 Expo app
  app/                       expo-router routes: (auth)/ (tabs)/bowls|chats|travel|studio|me
  src/api/ src/components/ src/features/<module>/ src/lib/
server/
  cmd/server/main.go
  internal/{auth,identity,metro,bowls,chat,realtime,travel,studio,safety,pii,notify,ai,admin,web}/
  migrations/                (explicit SQL for destructive changes; AutoMigrate for additive only)
  seed/metro/                GTFS-derived station and line data per city
admin/                       React + Vite moderator console, built into server embed
```

---

## 10. Integrations

| System | Purpose | Auto-executes? |
|---|---|---|
| MSG91 / Twilio | OTP, SOS SMS | Yes (user-initiated) |
| Expo Push (APNs/FCM) | Notifications | Yes |
| LiveKit | Audio rooms | Yes (server issues tokens) |
| Cloudflare R2 | Media | Yes |
| Anthropic API | PII second pass, moderation triage, icebreakers, topic ideas | **Recommends only.** Flags go to a human queue |
| NSFW image classifier | Image gate | Blocks visibility until it passes; humans review appeals |
| PostHog (self-host) or own events table | Product analytics with no PII | Yes |

---

## 11. Explicitly out of scope (v1)

Dating/swipe mechanics, profile photos, public real names, maps and live GPS tracking, payments/tipping for hosts, audio recording/replay, video, voice notes in chat, web client for end users, cab/auto pooling, cities beyond Delhi NCR at launch.
