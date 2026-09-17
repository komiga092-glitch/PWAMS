# Sinhala (si) Translation Review Checklist

**Status: BEST-EFFORT / MACHINE-ASSISTED — NOT native-speaker reviewed.**

Per SRS section 32, PWAMS ships with English, Tamil, and Sinhala UI
languages. The Tamil (`ta`) strings were written carefully, but the Sinhala
(`si`) strings are best-effort translations produced without a native
Sinhala speaker on the team. **They must be reviewed by a native Sinhala
speaker before this product is exposed to real NGO field users.**

## What to review

All strings live in `internal/i18n/locales/si.json` (730 keys, one flat
`"key": "value"` map). The English reference is `internal/i18n/locales/en.json`.

### Priority 1 — always-visible chrome (review first)

- `common.*` — buttons (save/cancel/edit/delete/view/search), statuses
  (Active/Disabled/Locked/Pending), pagination (Prev/Next/Page/of).
- `nav.*` — every sidebar label (Dashboard, Care Seekers, Students,
  Donors, Donations, Aid Requests, Loans, Repayments, Revenue, Users,
  Reports, Audit Logs, Logout, ...).
- `login.*` — the public login screen (org name, sign-in form labels).
  **NEW from the login redesign, review first:**
  - `login.org_name_ta` = "මක්කල් නලන් කාප්පගම්" — this is the Tamil
    organization name carried as literal brand text (identical in all
    three locale files, NOT translated per language). It will be shown
    to Sinhala users too; confirm the org's official Tamil name.
  - `login.org_name_en` = "ජනතා සුභසාධන සංගමය" — the org's English name
    as brand text (same literal everywhere); confirm the official name.
  - `login.user_id` / `login.email_or_username` = "පරිශීලක හැඳුනුම්පත"
    ("User ID" label) — confirm naturalness.
  - `reports.download_pdf` = "PDF බාගන්න" ("Download PDF" button on the
    Reports page) — confirm naturalness.
- `dashboard.*` — the first screen after login.

### Priority 2 — high-traffic workflow pages

- `students.*`, `donors.*`, `donations.*`, `aid_requests.*`, `loans.*`,
  `repayments.*`, `care.*` (Care Provided), `persons.*`,
  `revenue.*`, `users.*`.

### Priority 3 — secondary pages

- `notifications.*`, `messages.*`, `files.*`, `reports.*`,
  `audit.*`, `profile.*`, `role.*`, plus all remaining namespaces.

## Known rough spots (deliberately best-effort)

- `common.locked` = "අගුලු දමා ඇත" — account-state phrasing; confirm a
  natural UX term (vs. a literal "has been locked").
- `nav.dashboard` = "උපකරණ පුවරුව" — literal "instrument panel"; a
  shorter term (e.g. "මුල් පිටුව" / "ප්‍රධාන පුවරුව") may be preferred.
- `nav.care_seekers` = "රැකවරණ අපේක්ෂකයින්" — verify this matches the
  NGO's own terminology for beneficiaries who request care.
- `login.org_name` = "ජනතා සුභසාධන සංගම කළමනාකරණ පද්ධතිය" — the official
  product name translation; confirm with the org before launch.
- `care.*`, `reports.*` — domain terms (care types, report section
  headings) were transliterated from English; check naturalness.
- Status values (Pending/Confirmed/Cancelled/Approved/Rejected etc.) are
  translated for display only; the underlying values sent to the server
  remain English identifiers and must not be changed.

## How translations are wired

- Go side: `internal/i18n/i18n.go` loads the three JSON files at startup
  (`//go:embed locales/*.json`) and exposes `T(lang, key)` with fallback
  English → raw key, so a missing/unfinished translation degrades
  gracefully and never breaks a page.
- Middleware: `internal/middleware/locale.go` resolves `?lang=` query
  param → `pwams_lang` cookie → default `en`, and stores it in the Gin
  context as `lang`.
- Templates call `{{t .Lang "key"}}`; the login (templ) component calls
  `i18n.T(lang, "key")` directly; JS on the Care Provided page receives
  translations through an `I18N` object rendered into the page.

## Process

1. Review/fix values in `internal/i18n/locales/si.json` (keep keys
   identical to `en.json` — run `go run ./cmd/keyparity` after editing to
   confirm all three locales still match).
2. Rebuild: `go build ./internal/... ./cmd/... ./web/...`.
3. Click through the app with `?lang=si` (cookie persists) and verify
   layout does not break with longer/shorter Sinhala strings.
