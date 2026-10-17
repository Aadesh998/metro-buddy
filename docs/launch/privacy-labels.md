# Privacy labels

## Apple App Privacy

| Data type | Collected | Linked to user | Tracking | Purpose |
|---|---|---|---|---|
| Phone number | Yes | Yes | No | App functionality (login), safety (SOS to contacts the user adds) |
| Other user content (posts, comments, messages, bio) | Yes | Yes | No | App functionality, moderation |
| Photos (only images the user attaches) | Yes | Yes | No | App functionality |
| Coarse location | No (user picks stations; no GPS) | — | — | — |
| Precise location | No | — | — | — |
| Contacts | Yes (only trusted contacts the user types in) | Yes | No | App functionality (SOS) |
| Identifiers (push token, device ID) | Yes | Yes | No | App functionality (notifications) |
| Usage data (product interaction) | Yes, only with Analytics consent | No | No | Analytics |
| Other data: age (birth year), gender (optional) | Yes | Yes | No | Age verification, matching filters the user sets |

No tracking: no ad SDKs, no data brokers, no cross-app identifiers.

## Google Play Data safety

- **Data shared with third parties:** none for advertising. Processors only: SMS (MSG91/Twilio) for OTP and SOS, Expo for push, LiveKit for live audio, Anthropic for optional AI safety checks (consent-gated), Cloudflare R2 for images.
- **Data encrypted in transit:** yes (TLS).
- **Users can request deletion:** yes, in-app (Me → Privacy & your data → Delete account), completed within 30 days.
- **Collected:** phone number; user-generated content; photos the user attaches; app activity (only with analytics consent); device identifiers (push token); age and optional gender.
- **Location:** not collected.
