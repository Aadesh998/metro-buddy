# Building a "___OS" web app: tech stack & functional spec template

The web-app sibling of `app-template-spec.md` (the Wails desktop template).
Same spirit, adapted for multi-tenant SaaS: a separate Go API + React/Vite
frontend, Postgres in production (SQLite for local dev/tests), deployed to
Railway. Copy this file into the new project and fill in the bracketed
sections at the bottom — the tech stack and principles are meant to carry
over as-is unless there's a specific reason to deviate.

## What's different from the desktop template, and why

Collabot is a single-user local desktop tool — its two defining moves
("files on disk are truth," "a workspace is just a folder you point the app
at") don't survive contact with multi-tenant SaaS, where there's no local
filesystem to call durable and many strangers' data has to stay isolated.
Everything else — the Go service-boundary pattern, GORM-modeled entities,
the AI agent harness, the React component conventions — carries over
directly. The adaptations:

| Collabot (desktop) | This template (web) |
|---|---|
| Files on disk are the source of truth; SQLite is a disposable index | The database *is* the source of truth (no local disk to fall back on across deploys/restarts/scaling); files that need to exist (attachments, exports) go to object storage |
| "Workspace" = a folder you point the app at | "Workspace" = a tenant/org row in the database; switching workspace = switching which org your session is scoped to, not switching folders |
| No auth — one local user | Auth is mandatory — sessions, tenant isolation on every query |
| One process, one user, no scaling concerns | Stateless API process (any request can hit any instance), state lives in Postgres/object storage only |

## Tech stack

| Layer | Choice | Why |
|---|---|---|
| Backend | Go 1.26+, [Echo v4](https://echo.labstack.com) | Same language/patterns as the desktop template; Echo's built-in middleware (logging, recover, CORS, JWT) covers most of what a JSON API needs out of the box |
| ORM | [GORM](https://gorm.io) — `gorm.io/driver/postgres` in production, `gorm.io/driver/sqlite` for local dev/tests | Same models, same code, different driver per environment (see Environments below) |
| Database (prod) | Postgres (Railway-managed) | Real concurrent writes, real backups, the standard choice for multi-tenant SaaS |
| Database (local/test) | SQLite (file or `:memory:`) | Zero setup for local dev and unit tests — GORM's model layer is the same either way, only the driver differs |
| Frontend | React 18 + TypeScript + Vite | Same as desktop — a plain SPA, not Next.js, since the backend is a separate Go API rather than co-located API routes |
| Styling | Tailwind CSS + shadcn/radix-ui primitives | Identical to desktop — the component conventions transfer directly |
| Icons | lucide-react | Same as desktop |
| Auth | Passwordless magic-link email (or WebAuthn/passkeys) as the primary login flow; session via JWT in an `httpOnly`, `Secure`, `SameSite=Lax` cookie — short-lived access token + rotating refresh token | Stateless — any API instance can validate a request without a shared session store, which matters once Railway is running more than one instance. No password means no phishing/reuse/breach risk to design around. Reach for Clerk/Auth0/WorkOS instead if you'd rather not own auth at all (real option — outsourcing auth is a legitimate simplicity win for a small team, at the cost of a per-user fee and less control over the login UI). |
| File storage | S3-compatible object storage (Cloudflare R2 or AWS S3) for anything file-like (attachments, exports) | No durable local disk to lean on across deploys — this is the direct replacement for Collabot's `data/contexts/.../attachments/` folder |
| AI provider | Anthropic API (Claude), same direct HTTP client pattern as desktop | No change — see the AI harness section below for the one thing that *does* need to change (per-tenant metering) |
| Hosting | [Railway](https://railway.app) | Auto-detects and builds a Go app via Nixpacks — no Dockerfile needed for v1. See Deployment below for the recommended single-service shape. |

**Deliberately not used, unless a specific need arises:**
- No Next.js / server-side rendering — this is a SPA behind a separate API; add SSR later only if SEO or first-paint time on marketing pages actually demands it (and even then, consider a *separate* static marketing site rather than SSR-ing the whole app).
- No microservices — one Go binary, one Postgres database, until there's a concrete, measured reason to split something out.
- No frontend state library, no drag-and-drop library, no frontend test framework — same reasoning as the desktop template.
- No Kubernetes — Railway's own scaling is enough until you have a specific reason it isn't.

## Architectural principles

1. **The database is truth; nothing durable lives on local disk.** Every API instance is disposable and stateless — a request can land on any instance, and an instance can be killed/replaced at any time. Anything that needs to survive belongs in Postgres or object storage, never on the container's local filesystem (even a Railway volume, unless you've deliberately chosen to pin a single instance for something that genuinely needs it — treat that as an exception requiring justification, not a default).
2. **Tenant isolation is enforced at the query layer, not trusted from the client.** Every tenant-scoped GORM model gets a `TenantID` (or `OrgID`) column, and every query touching it is scoped through a helper that injects that condition — never trust a tenant ID that arrives in a request body/query param without cross-checking it against the authenticated session's tenant. A GORM [global query scope](https://gorm.io/docs/scopes.html) or a wrapped `*gorm.DB` that always includes `WHERE tenant_id = ?` is worth the upfront cost of every model going through it.
3. **Auth first, features second.** Don't build tenant-scoped features against an unauthenticated stub — get real sessions and tenant isolation working before the first real feature, or every feature built before that point has to be retrofitted.
4. **The AI recommends, the user decides** (same as desktop) — no auto-execution of destructive or external-facing actions without an explicit human step. Log every AI-driven write to a per-tenant audit trail.
5. **Progressive disclosure to the AI** (same as desktop) — dynamic system prompts scoped to what's relevant, not everything the tenant owns; tools to fetch more detail on demand rather than front-loading it.
6. **Everything reversible where it's destructive** (same as desktop) — undo toast for low-stakes deletes, confirmation for anything that removes a whole entity or affects other tenant members.
7. **Manual ordering matters wherever judgment beats automation** (same as desktop).
8. **Budget/rate-limits are per-tenant, not just global.** A single tenant hammering the AI (or the API in general) shouldn't be able to degrade service for everyone else — track usage and enforce limits scoped to `TenantID`, not just a process-wide counter.
9. **Small, reviewable increments** (same as desktop).

## Data model patterns worth reusing

- **Every tenant-scoped model gets `TenantID` + the usual GORM base fields**: `ID`, `TenantID \`gorm:"index"\``, `CreatedAt`, `UpdatedAt`, soft-delete via `DeletedAt gorm.DeletedAt \`gorm:"index"\`` if you want undo-by-restore rather than hard deletes. See Security & performance guidelines below for the recommended `ID` strategy (short internal PK + Sqids at the boundary, vs. UUID) — don't leak row counts or make IDs guessable across tenants either way.
- **Hierarchical entities** (if the core object nests, like Collabot's contexts): same shape as desktop — `Path`, `ParentID` (a real foreign key here, not a path string, since there's no filesystem to mirror) — plus `TenantID`. Ordering via a `SortOrder int` column, renumbered per-sibling-group on reorder.
- **Users and Tenants are separate models, joined by a membership table** (`TenantMember{TenantID, UserID, Role}`) — a user can belong to more than one tenant (the "workspace switcher" from the desktop template becomes "which tenant is this session currently scoped to," driven by this membership table, not a folder registry).
- **A per-tenant settings model** (`TenantSettings{TenantID \`gorm:"primaryKey"\`, ...}`) for anything that isn't a domain entity — AI model choice, budget caps, integration toggles. Never store API keys tenant-provides in plaintext — encrypt at rest (e.g. via a KMS-backed envelope key, or at minimum `age`/`nacl/secretbox` with a key from an environment secret, never checked into the repo).
- **An append-only audit log per tenant** (a real table, `AuditLog{TenantID, ActorID, Verb, Description, CreatedAt}`, not a Markdown file this time) — same purpose as Collabot's `log.md`: the safety net that makes "no approval gate" for AI writes acceptable.
- **Plain-text line format with inline tokens** (tasks, checklists) still applies if the app has that kind of entity — store the raw text in a `TEXT` column rather than trying to model every token as its own database column; parse/serialize on the way in/out, same as the desktop pattern.

## Security & performance guidelines

### Security

- **Never expose raw database primary keys to the frontend.** Keep internal PKs as a fast auto-increment `BIGINT` (compact foreign keys, good insert locality — better write performance than random UUIDs on a busy table) and encode/decode them at the API boundary with [Sqids](https://sqids.org/go) (the modern, non-patented successor to Hashids) — internal ID `42` becomes something like `Uc3fjr` in every response and decodes back to `42` on the way in. This is obfuscation, not encryption — don't rely on it to keep an ID secret from someone determined to decode it — but it kills casual enumeration ("what's at ID 43?") and stops leaking row counts/growth rate. If you'd rather skip the encode/decode step entirely, use UUIDv7 (or ULID) as the actual PK instead — no obfuscation layer needed since the PK itself isn't sequential, at the cost of a larger index and slightly worse insert locality than a `BIGINT`. Pick one of the two approaches deliberately; don't do neither.
- **Separate DTOs from persistence models — never return (or accept) a GORM model directly in a handler.** Every endpoint gets its own request/response struct (`CreateContextRequest`, `ContextResponse`, ...) with an explicit mapping function to/from the GORM model. This is what actually *enforces* the ID-encoding rule above (the DTO's `ID` field is a `string`, the model's is an `int64` — the mapping function is the one place that ever converts between them), and it's what stops an internal-only column added later (a soft-delete flag, an `internal_notes` field a support tool writes) from silently leaking into the API the moment someone adds it to the model.
- **Passwordless-first login.** Prefer a magic-link email (a short-lived, single-use signed token emailed on request — nothing to phish, reuse across sites, or leak in a password-database breach) or WebAuthn/passkeys (stronger still, no email round-trip, phishing-resistant by design) over password-based login as the default. If you do keep passwords (passkeys are more upfront implementation effort than a v1 might warrant), make TOTP-based 2FA (`pquerna/otp` or similar) mandatory on top — don't ship password-only auth for a multi-tenant SaaS holding real user data.
- **Rate limit every auth endpoint, not just the general API.** Per-IP and per-account limits on magic-link requests and login attempts prevent enumeration/brute-force; per-tenant limits on the general API (Architectural principle 8) stop one noisy tenant degrading service for everyone else.
- **Validate and re-verify, never trust client-supplied input** — struct-tag validation (`go-playground/validator`) on every DTO before it touches a query, and critically: re-derive the tenant ID from the authenticated session on every request, never from a client-supplied field (Architectural principle 2) — cross-tenant data access via a trusted client-supplied ID is the single most common multi-tenant SaaS vulnerability.
- **Lock down CORS and security headers** — `Access-Control-Allow-Origin` set to your exact deployed frontend URL (never `*`, especially with cookies in play), plus `Content-Security-Policy`, `X-Content-Type-Options: nosniff`, and `Strict-Transport-Security`.
- **Secrets never touch the frontend or logs** — API keys, JWT signing secret, DB credentials from Railway env vars only; structured logging with an explicit denylist of fields that never get logged (passwords, tokens, tenant-provided API keys).

### Performance

- **Paginate every list endpoint, no exceptions** — even ones "that will never have many rows." Prefer cursor/keyset pagination (`WHERE id > ? ORDER BY id LIMIT ?`) over offset-based (`OFFSET ? LIMIT ?`) once a table can grow large — offset pagination gets slower the deeper you page, and rows can shift between pages if the underlying data changes mid-scroll.
- **Index every foreign key and every column used in a `WHERE`/`ORDER BY`** — `TenantID` on every tenant-scoped table (it's in almost every query's `WHERE` clause), `SortOrder` on anything paginated in order, whatever column backs the cursor above.
- **Avoid N+1 queries** — GORM's `Preload`/`Joins` for anything that fans out (a list of entities, then a separate query per row for each one's children) instead of looping and querying per-row. The single most common GORM performance foot-gun.
- **Set connection pool limits explicitly** — `sql.DB.SetMaxOpenConns`/`SetMaxIdleConns` sized to what your Postgres plan actually allows; an unbounded pool is how one runaway endpoint takes the database down for every tenant at once.
- **Cache what's genuinely expensive and safe to serve a few seconds stale** — AI-generated content, aggregate/rollup views. A simple in-process TTL cache is enough until there's a concrete reason for Redis (multiple API instances needing a shared cache, or values too large to duplicate per-instance).

## OWASP Top 10 checklist

A pass over the [OWASP Top 10 (2021)](https://owasp.org/Top10/) against everything above — most categories are already covered by the guidelines earlier in this doc; the ones below are the gaps worth calling out explicitly, plus a pointer back to where each *is* already handled.

| # | Risk | Where this template handles it |
|---|---|---|
| A01 | **Broken Access Control** | Tenant re-verification on every request (Security guidelines, Architectural principle 2). One more thing to hold onto beyond tenant match: **IDOR** — even inside the right tenant, re-check that the authenticated user is actually allowed to touch *this specific* resource (e.g. a `viewer` role hitting an admin-only entity) — tenant isolation alone doesn't imply role/ownership checks are done. |
| A02 | **Cryptographic Failures** | TLS by default on Railway; secrets never committed (Security guidelines); tenant-provided API keys encrypted at rest (Data model patterns). If you keep password auth at all, `bcrypt`/`argon2id` only — never MD5/SHA1/unsalted hashes. |
| A03 | **Injection** | GORM parameterizes queries automatically — the actual risk is a hand-written `Where("... " + userInput)` string-concatenation escape hatch; don't do that, use GORM's `?`-placeholder methods even for raw SQL. On the frontend: React/JSX auto-escapes by default — the risk is `dangerouslySetInnerHTML` or rendering AI-generated/user-generated Markdown as raw HTML. If you render Markdown (chat replies, notes), run it through `rehype-sanitize` (or equivalent) rather than trusting the source — an AI reply that happens to echo back a `<script>` tag a tenant pasted in is still attacker-controlled content once it's on the page. |
| A04 | **Insecure Design** | Not really automatable — the closest thing is a habit: for each new feature, spend one minute asking "what does a malicious tenant do with this" before building it (classic ones: an ID field that turns out to accept another tenant's ID, a webhook endpoint with no signature check, a file upload with no size/type limit). |
| A05 | **Security Misconfiguration** | CORS/security headers (Security guidelines). Add: never return a raw Go panic/stack trace or GORM error string to the client in production — a generic `500` + a server-side log with the real detail; disable any framework debug/pretty-error mode outside local dev; don't ship default/example credentials anywhere reachable. |
| A06 | **Vulnerable and Outdated Components** | Not yet covered — add it: `go list -m -u all` and `npm audit` (or Dependabot/Renovate) in CI, failing the build on high-severity findings rather than just reporting them. Pin dependency versions (`go.sum`/`package-lock.json` committed, which they are by default). |
| A07 | **Identification and Authentication Failures** | Passwordless-first, mandatory 2FA if passwords are kept, rate-limited auth endpoints, rotating refresh tokens (Auth pattern below). Add: regenerate the session/token on every login (don't reuse a pre-login session ID — session fixation), and lock/backoff an account after repeated failed attempts even with rate limiting in place. |
| A08 | **Software and Data Integrity Failures** | Not yet covered — add it if you build any integration with inbound webhooks (verify the provider's signature on every webhook before trusting the payload — this matters the moment MarketingOS-style features post to or receive callbacks from an external platform); pin CI action versions by commit SHA, not a mutable tag; never pipe a remote script straight into a shell in a build step. |
| A09 | **Security Logging and Monitoring Failures** | The per-tenant audit log (Data model patterns) covers *AI/user-driven writes*. Add a **separate security-event log** — failed logins, permission denials, rate-limit trips — since mixing those into the same audit trail as "founder renamed a context" makes real incidents hard to spot; alert on spikes (e.g. failed-login rate) rather than only logging them. |
| A10 | **Server-Side Request Forgery (SSRF)** | **Directly relevant if this app fetches any user-supplied URL server-side** (a link-preview feature, an AI `web_fetch` tool, a webhook-URL "test connection" button — Collabot's own `internal/linkpreview` package is exactly this pattern). Beyond validating the scheme is `http(s)` (already necessary, not sufficient): resolve the hostname and reject requests to private/reserved IP ranges (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `127.0.0.0/8`, `169.254.0.0/16` — that last one is the cloud metadata endpoint, `169.254.169.254`, that leaks cloud credentials if an app can be tricked into fetching it) and `localhost`; re-validate after following any redirect rather than trusting the original URL's check (a redirect is exactly how an SSRF filter gets bypassed); set a strict timeout and response-size cap (already in place for the link-preview use case). |

## Auth pattern (concrete recommendation)

- **Signup/login**: magic-link email as the primary flow (see Security guidelines above) — a signed, single-use, short-lived (~15 min) token emailed on request, exchanged for a session on click. OAuth (Google/GitHub) as an alternative entry point via a minimal handler; add password + mandatory TOTP 2FA only if you have a specific reason email-based login doesn't fit.
- **Session**: on successful login, issue a short-lived (e.g. 15 min) JWT access token *and* a longer-lived (e.g. 30 day) opaque refresh token, both in `httpOnly Secure SameSite=Lax` cookies. Refresh token is a random string stored (hashed) in the database against the user, rotated on every use (detect reuse = revoke the whole session family — the standard refresh-token-rotation defense against token theft).
- **Every authenticated request**: middleware validates the access token, loads the current tenant from a `X-Tenant-ID` header or the JWT's claim (whichever the frontend sends — pick one and be consistent), and attaches both `UserID` and `TenantID` to the request context for every downstream handler/query.
- **Don't build this from scratch if you'd rather not** — Clerk, WorkOS, or Auth0 are legitimate choices if owning login/session infra isn't where you want to spend time; they cost per-user but remove an entire category of security surface area to get right yourself, and most support passwordless/passkeys out of the box.

## AI agent harness pattern

Identical to the desktop template's harness pattern (dynamic system prompts, hand-rolled tool-use loop, prompt caching on system + last message, compaction at ~25% of context window, throttled SSE streaming, response token budget derived from the model's real max output, read-only tools by default) — see `app-template-spec.md` for the details. The one addition for multi-tenant SaaS:

- **Usage and budget tracking is per-tenant.** Every API call's cost gets recorded against `TenantID`, not just a global usage log — and budget enforcement (daily/monthly caps) checks that tenant's spend, not the whole app's.
- **API key strategy**: either you hold one Anthropic key and meter/bill tenants for usage (simpler UX, you carry the cost risk and need billing), or tenants bring their own key (stored encrypted per-tenant, zero cost risk to you, more setup friction for them). Pick one deliberately — don't leave it ambiguous.

## Frontend patterns worth reusing

Identical to the desktop template — lifted state for anything that must survive a component unmounting, shared engine components with I/O behind callback props, inline toggle panels instead of a Popover primitive until you have 3+, hover-reveal actions, `window.confirm` for destructive actions. The one new concern:

- **A tenant/workspace switcher in the header**, backed by the membership table above — same UX shape as Collabot's `WorkspaceSwitcher.tsx`, but it calls an API endpoint to switch the session's active tenant instead of relaunching a process pointed at a different folder.

## SEO / public pages (Go templates + htmx)

The React/Vite SPA is the right tool for the authenticated app — but it's the wrong tool for anything that needs to be crawled, indexed, or unfurl nicely when shared: the marketing homepage, `/pricing`, a blog, public docs, a shared/public view of a tenant's content. A client-rendered SPA either serves crawlers an empty `<div id="root">` (bad for indexing, even though major crawlers do execute JS these days, it's still slower and less reliable than real HTML) and gives social platforms nothing to unfurl without extra work. Don't try to make the SPA handle this — run these as genuinely server-rendered pages, in the same Go binary, on a separate route group.

- **Templating**: Go's stdlib `html/template` for a v1 (zero dependencies, auto-escapes by default — the same injection protection React's JSX gives you, just server-side) — reach for [templ](https://templ.guide) instead once the page count/complexity grows enough that type-safe, component-style templates (composable, checked at compile time) are worth the extra build step.
- **Interactivity via htmx, not React, on these pages.** A pricing toggle, a newsletter signup form, a live search box on a blog — `hx-get`/`hx-post`/`hx-swap` fetch and swap in a fragment of server-rendered HTML, no client-side JS framework, no separate API contract to maintain for these pages. This keeps public pages fast (no framework bundle to download) and genuinely simple — htmx's whole model is "the server still owns the HTML," which fits a Go backend far better than reimplementing the same page in React.
- **Route split, one server**: mount the public/SEO route group (`/`, `/pricing`, `/blog/*`, `/docs/*`) and the SPA (`/app/*`, plus its `/api/*` backend) in the same Echo instance — the public pages should not require the SPA's JS bundle at all, and the SPA should never need to be server-rendered. Keep the boundary clean: nothing under `/app` is crawled (it's behind auth anyway, and shouldn't be indexed — send `X-Robots-Tag: noindex` on it just in case), nothing under the public route group depends on React.
- **Meta tags and social unfurling, per page, server-rendered into `<head>`** — title, meta description, canonical URL, and Open Graph tags (`og:title`, `og:description`, `og:image`) so a link to your own pricing page or blog post unfurls correctly on Slack/X/LinkedIn — the exact same fields Collabot's own `internal/linkpreview` fetches *from other people's* pages; here you're the one publishing them. A per-page template partial for this, filled from whatever content model backs the page, is enough — no need for a dedicated "SEO settings" UI unless non-engineers need to edit these independently.
- **Structured data** (JSON-LD in a `<script type="application/ld+json">` block) for anything that benefits from rich snippets — articles, FAQs, pricing. Optional, but cheap to add once the template layout exists.
- **`sitemap.xml` and `robots.txt`** served from the public route group — generate the sitemap from whatever drives the public pages (a list of blog posts, docs pages) rather than hand-maintaining it, so it can't silently go stale.
- **Cache aggressively.** These pages are the same for every visitor (unlike the per-tenant app) and change rarely — set `Cache-Control` headers generously and let a CDN (Cloudflare in front of Railway, or Railway's own edge caching if available) serve most requests without hitting the Go process at all. This is also just good performance hygiene for the pages that actually matter most for first impressions (a slow marketing page loses visitors before they ever see the product).

## Deployment (Railway)

Recommended shape for v1: **one Railway service**, not two. The Go binary embeds the built frontend (`//go:embed all:frontend/dist`, exactly like Collabot embeds its Wails frontend) and serves the SPA, the server-rendered public/SEO pages, and the `/api/*` routes all from the same process — no CORS to configure, no second service to deploy/monitor, and Railway's Nixpacks builder handles a Go binary with zero config.

```
cmd/server/main.go       # echo.New(), mounts routes/middleware
internal/
  <domain>/              # GORM models + query methods, mirrors <domain>store from the desktop template
  auth/                  # session/JWT issuance and validation middleware
  agent/                 # AI harness — same shape as desktop
  api/                   # HTTP handlers for the SPA, one file per resource
  web/                   # server-rendered public pages — html/template (or templ) + htmx, mounted outside /app and /api
frontend/
  dist/                  # built by `npm run build`, embedded into the Go binary, served under /app
Procfile or railway.toml # build: npm run build (frontend) && go build ./cmd/server; start: ./server
```

Build order (mirrors the desktop template's CI order):
```bash
cd frontend && npm ci && npm run build   # produces frontend/dist, which go:embed picks up
cd .. && go build ./cmd/server            # embeds frontend/dist into the binary
go test ./...
```

**Environments**: a single `DATABASE_URL` env var, driver picked at startup based on its scheme (`postgres://` → `gorm.io/driver/postgres`, empty/`file:` → `gorm.io/driver/sqlite`) — so local dev and CI run against SQLite with zero setup, and Railway's Postgres plugin just works by setting the env var. Run `AutoMigrate` on startup for both; for production, consider gating destructive schema changes (column drops, type changes) behind an explicit migration step rather than relying on `AutoMigrate` alone once the schema stabilizes.

**Secrets**: Anthropic API key, JWT signing secret, object storage credentials — all Railway environment variables, never committed. Encrypt any *tenant-provided* secrets (their own API keys, OAuth tokens for integrations) at rest with a key that itself comes from a Railway secret.

---

## Functional spec — [APP NAME]

*Same structure as the desktop template's functional spec — fill in per app.*

### Idea
[One paragraph: what is this app, who is it for, what problem does it solve, what does it deliberately NOT do.]

### Core entity
[What's the nested/central "thing" this app organizes? Define the tree/relationship shape and how it maps to tenants.]

### Features
[Bullet list, roughly in build order. Which of the reusable patterns above apply?]

### Integrations
[What external systems does this need to talk to — and does it recommend or auto-execute, per principle 4 above?]

### Explicitly out of scope
[What are you deliberately not building, at least for v1.]
