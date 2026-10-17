# Launch checklist (T081)

Status of each item. Store submissions need the Apple Developer and Google Play accounts.

| Item | State | Where |
|---|---|---|
| Store listing copy (both stores) | Ready | `store-listing.md` |
| Apple privacy label / Google Data safety answers | Ready | `privacy-labels.md` |
| UGC policy compliance (App Store 1.2, Play UGC) | Ready, mapped to features | `ugc-compliance.md` |
| Age rating answers (18+) | Ready | `store-listing.md` |
| Load test | Passed locally (200 users, 0 errors) | `load-test.md` |
| Legal pages live (privacy, terms, guidelines, grievance) | Built, need lawyer review | `/privacy`, `/terms`, `/guidelines`, `/grievance` |
| Demo account for app review | To create on production | see below |
| `eas init`, signing credentials, builds | Needs Expo + store accounts | `apps/mobile/eas.json` |
| Railway production project + secrets | Needs Railway account | `.env.example`, `railway.toml` |
| MSG91 DLT templates (OTP + SOS) | Needs MSG91 account + DLT registration | `MSG91_*` env |
| LiveKit project | Needs LiveKit Cloud account | `LIVEKIT_*` env |
| Cloudflare R2 bucket + NSFW classifier endpoint | Needs accounts | `R2_*`, `NSFW_ENDPOINT` |

## App review demo account

Reviewers can't receive OTPs on Indian numbers. Before submitting, set `OTP_DEV_CODE` only for a single reviewer number via a dedicated build-time allow-list, or provide a reviewer phone through the review notes. Never enable `OTP_DEV_CODE` globally in production (the server refuses to start if you try).

## Pre-launch smoke test (production)

1. `GET /healthz` returns `{"db":"ok","redis":"ok"}`.
2. Sign up with a real number, finish onboarding, post in a bowl, report your own post from a second account, action it in `/admin`.
3. DM request → accept → chat on two phones; kill network on one, send, restore, confirm delivery.
4. Travel Now on two phones on the Yellow Line → match → accept both → trip → SOS with a trusted contact you control; confirm the SMS and the share page.
5. Push: confirm a reply notification arrives and quiet hours suppress it.
