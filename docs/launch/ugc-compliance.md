# UGC compliance

## App Store Guideline 1.2 / Google Play UGC policy

| Requirement | How Metro Buddy meets it |
|---|---|
| Filter objectionable material | PII masker on all UGC before storage (`server/internal/pii`), Claude Haiku second pass for obfuscated contact details, NSFW gate keeps images hidden until cleared |
| Mechanism to report offensive content, with timely response | ⋯ on every post, comment, message, profile, DM request and studio item opens the report sheet (`SafetySheet`); cases have severity-based SLAs (1h critical, 4h high, 24h medium) in the admin queue |
| Ability to block abusive users | Block from the same sheet; blocks are bidirectional across feeds, chats, matching and rooms (`safety.ExcludeBlocked`) |
| Published contact information | `/grievance` page and in-app Safety Center |
| Terms of use users must agree to | Shown at signup (phone screen) and linked in Settings |
| Remove users who post objectionable content | Moderator actions: remove, warn, 24h mute, suspend, ban (leads), all audit-logged; appeals reviewed by a different moderator |
| Age gate for mature/social apps | 18+ date-of-birth check before any social feature |

## IT Rules, 2021 (India)

| Rule | Implementation |
|---|---|
| Grievance Officer published | `/grievance` with contact and form |
| Acknowledge within 24h, resolve within 15 days | Ticket SLAs, overdue alerts job (`AlertOverdue`) |
| Remove non-consensual intimate imagery within 24h | NCII grievances get a 24h resolve SLA; `sexual` reports are critical (1h) |
| Notify users of action and allow appeal | Account notices + appeal flow (`/v1/me/moderation`, `/v1/appeals`) |
| Retain records for investigations | PII vault (encrypted, 90 days), moderation actions and audit logs (5 years) |

## DPDP Act, 2023

Itemised consent at signup and in Settings, data export, deletion within 30 days, Data Protection Officer contact on `/privacy`.
