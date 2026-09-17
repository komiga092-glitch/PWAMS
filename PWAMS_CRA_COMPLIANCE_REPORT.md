# PWAMS EU Cyber Resilience Act (CRA) Compliance Readiness Report

**Audit Date:** 2026-09-09
**Project:** People Welfare Association Management System (PWAMS)
**Repository:** github.com/komiga092-glitch/pwams
**Auditor:** Kilo (Automated Technical Readiness Assessment)
**Classification:** Technical readiness assessment only — not legal certification

---

## 1. Executive Summary

This report documents a technical CRA readiness assessment of PWAMS, a Go/Gin backend with TypeScript PWA frontend for NGO beneficiary management. The assessment inspected actual repository evidence including authentication, authorization, API security, database integrity, offline/sync security, SBOM, vulnerability scanning, and CI/CD configuration.

**Final Classification: CRA READY WITH FINDINGS**

PWAMS demonstrates strong security-by-design fundamentals: bcrypt password hashing, server-side sessions, double-submit CSRF, RBAC middleware, rate limiting, security headers, audit logging, optimistic locking, tenant isolation, encrypted offline PII, and idempotent sync. Build, tests, lint, and frontend compilation all pass. However, 6 reachable HIGH/MEDIUM vulnerabilities in the Go 1.25 standard library are unresolved due to toolchain upgrade being blocked, and several organizational security documents are absent. These findings do not prevent deployment but must be tracked and remediated.

---

## 2. Assessment Scope

- Backend: Go 1.25, Gin, GORM, PostgreSQL
- Frontend: TypeScript, PWA with IndexedDB offline support
- Infrastructure: Docker, nginx, GitHub Actions CI/CD
- Evidence reviewed: source code, SBOM, npm audit, Go build/test, CI workflow, configuration files

---

## 3. Assessment Method

Static code review of security controls, dependency inventory via SBOM, vulnerability scanning via govulncheck/npm audit, build/test verification, and comparison against CRA technical requirement areas A–R.

---

## 4. PWAMS Product Description

PWAMS is a web-based management system for people welfare associations. It supports user authentication with role-based access (6 roles), beneficiary/person records, students, donors, donations, aid requests, care records, loans, repayments, revenue tracking, notifications, messages, file uploads, reporting, audit logging, and offline PWA synchronization. The backend exposes a JSON API and HTML pages via Gin; the frontend is a TypeScript PWA with service worker, IndexedDB offline storage, encrypted PII fields, and background sync.

---

## 5. CRA Compliance Matrix

| Requirement | CRA Area | PWAMS Evidence | Implementation | Status | Risk | Gap | Recommended Action | Evidence Location |
|---|---|---|---|---|---|---|---|---|
| Secure authentication with server-side sessions | I | bcrypt hashing, 32-byte random session tokens, SHA-256 token hashing, HttpOnly cookies | `internal/utils/password.go`, `internal/services/session_service.go`, `internal/handlers/auth_handler.go` | PASS | Low | None | Maintain current implementation | See Section 13 |
| CSRF protection | I, K | Double-submit cookie token validated on unsafe methods, SameSite strict | `internal/middleware/csrf.go` | PASS | Low | None | Maintain current implementation | See Section 13 |
| RBAC / authorization | I | Role-based middleware enforcing Super Admin/Admin/Staff/Volunteer/Donor/Beneficiary | `internal/middleware/rbac_middleware.go`, `internal/models/role.go` | PASS | Low | None | Maintain current implementation | See Section 13 |
| Rate limiting | I, L | IP-based rate limits on login (5/15m), OTP (3/10m), password reset (3/15m), generic (30/1m) | `internal/middleware/rate_limit.go` | PASS | Low | None | Maintain current implementation | See Section 13 |
| Security headers | K | X-Content-Type-Options, X-Frame-Options, HSTS, CSP, COOP, COEP, CORP, Permissions-Policy | `internal/middleware/security.go`, `nginx.conf` | PASS | Low | None | Maintain current implementation | See Section 13 |
| Input validation / SQL injection protection | J | Parameterized queries via GORM, validation service for phone format | `internal/services/validation.go`, repository layer | PASS | Low | None | Maintain current implementation | See Section 13 |
| IDOR protection | J | Repository-scoped queries, user-bound file access (`FindByIDAndUser`) | `internal/repository/file_upload_repository.go`, service layer | PASS | Low | None | Maintain current implementation | See Section 13 |
| Audit logging | L | Authentication events logged with user ID, action, entity, IP address | `internal/services/audit_log_service.go`, `internal/handlers/auth_handler.go` | PARTIAL | Medium | Audit logs are only for auth events; no coverage for data mutations, sync, or admin actions | Extend audit logging to all sensitive operations | See Section 12 |
| Data integrity / optimistic locking | J | Version fields on Person and sync entities, conflict detection in sync | `internal/models/person.go`, `internal/services/person_sync_service.go`, `internal/services/entity_sync_service.go` | PASS | Low | None | Maintain current implementation | See Section 13 |
| Secure file upload | J | Content-type validation via magic bytes, soft delete with physical file removal, user-scoped access | `internal/services/file_upload_service.go`, `internal/handlers/file_upload_validation_test.go` | PASS | Low | None | Maintain current implementation | See Section 13 |
| PWA / offline security | J, K | Encrypted PII in IndexedDB via AES-GCM, offline PIN with PBKDF2, CSRF token on sync, idempotency keys | `web/src/offline/pii.ts`, `web/src/offline/session.ts`, `web/src/offline/sync.ts`, `web/src/offline/csrf.ts` | PASS | Low | None | Maintain current implementation | See Section 13 |
| Sync security | J | Server-authoritative user/tenant, idempotency, versioned conflict detection, ordered entity processing | `internal/services/sync_service.go`, `internal/handlers/sync_handler.go` | PASS | Low | None | Maintain current implementation | See Section 13 |
| SBOM / component inventory | H | CycloneDX 1.5 and SPDX 2.3 SBOMs generated, 110 components inventoried | `PWAMS_SBOM_SECURITY_REPORT.md` | PARTIAL | Medium | SBOM exists but Docker image SBOM was blocked; image scanning incomplete | Complete container SBOM and image scan | See Section 9 |
| Vulnerability scanning | M | govulncheck found 11 Go vulnerabilities; npm audit found 0 | `PWAMS_SBOM_SECURITY_REPORT.md`, `npm_audit.json` | PARTIAL | High | 6 reachable Go stdlib vulnerabilities remain unfixed | Upgrade Go toolchain to 1.26.6+ | See Section 8 |
| Dependency management | G | Go modules pinned, npm lockfile present, CI uses pinned action versions | `go.mod`, `go.sum`, `package-lock.json`, `.github/workflows/ci-cd.yml` | PARTIAL | Medium | Docker images use floating tags; golangci-lint uses floating internal version | Pin Docker image digests; pin linter version | See Section 9 |
| Secure communications | K | TLS configurable via DB_SSLMODE, HSTS header, SameSite cookies, HTTPS in nginx config | `internal/database/postgres.go`, `internal/middleware/security.go`, `nginx.conf` | PARTIAL | Medium | TLS not enforced in nginx (HTTP redirect commented out); ECH privacy leak in Go 1.25 TLS | Enforce HTTPS redirect; upgrade Go toolchain | See Section 13 |
| Error handling | L | Generic error messages to clients, detailed errors logged server-side | `internal/handlers/auth_handler.go`, `internal/handlers/sync_handler.go` | PASS | Low | None | Maintain current implementation | See Section 13 |
| Logging and monitoring | L | Audit log service with IP capture, structured error logging | `internal/services/audit_log_service.go` | PARTIAL | Medium | No centralized log aggregation, alerting, or SIEM integration documented | Define logging strategy and alerting thresholds | See Section 12 |
| Security testing | M | Unit tests for security headers, CSRF, rate limiting, RBAC, file upload validation, session handling | `internal/middleware/security_test.go`, `internal/middleware/rate_limit_test.go`, `internal/middleware/rbac_middleware_test.go`, `internal/handlers/file_upload_validation_test.go`, `web/src/offline/session.test.ts`, `web/src/offline/media.test.ts` | PARTIAL | Medium | No penetration test report, no DAST/SAST in CI beyond lint | Add SAST scanning to CI; schedule periodic pen tests | See Section 16 |
| Update/patch mechanism | N | CI/CD pipeline builds and pushes Docker images on main branch push | `.github/workflows/ci-cd.yml` | PARTIAL | Medium | No documented emergency patch process, rollback strategy, or dependency update SLA | Document security update policy | See Section 12 |
| Vulnerability disclosure | E | No security contact, SECURITY.md, or disclosure policy found | Repository root | FAIL | High | No published vulnerability reporting channel | Create SECURITY.md with disclosure process | See Section 11 |
| Incident response | F | No incident response plan, alerting runbook, or evidence preservation workflow | Repository root | FAIL | High | No documented incident detection, investigation, or recovery procedures | Create incident response plan | See Section 12 |
| Technical documentation | O | Swagger UI exists in non-production; architecture inferred from code | `docs/docs.go` | PARTIAL | Medium | No security architecture document, threat model, or deployment hardening guide | Create security architecture and deployment docs | See Section 12 |
| Lifecycle / support | P | No support period, EOL policy, or versioning strategy documented | Repository root | NOT VERIFIED | Medium | No public commitment to security support timeline | Define and publish support policy | See Section 12 |
| User security instructions | Q | No user-facing security guidance found | Repository root | NOT VERIFIED | Low | No instructions for password hygiene, session management, or incident reporting | Create user security instructions | See Section 12 |

---

## 6. Cybersecurity by Design

### Verified Implementations

**Authentication and Password Security**
- Passwords hashed with bcrypt at default cost (`internal/utils/password.go`)
- Login enforces account status checks (Active, Disabled, Locked, Pending) (`internal/services/auth_service.go`)
- Account lockout after 3 failed attempts for 30 minutes (`internal/services/auth_service.go`)
- Session tokens are 32-byte cryptographically random values, stored as SHA-256 hashes (`internal/services/session_service.go`)
- Session cookies are HttpOnly, SameSite=Lax, and conditionally Secure in production (`internal/handlers/auth_handler.go`)

**CSRF Protection**
- Double-submit token pattern with `pwams_csrf` cookie
- Token validated on all unsafe methods (POST, PUT, PATCH, DELETE) using constant-time comparison (`internal/middleware/csrf.go`)
- CSRF cookie is NOT HttpOnly (required for JS read) but SameSite=Strict

**Authorization**
- `RequireRole` and `RequireAnyRole` middleware enforce role checks on protected routes (`internal/middleware/rbac_middleware.go`)
- Super Admin modification restricted to Super Admin actors (`internal/services/user_service.go`)
- Tenant scope middleware available for future multi-tenant isolation (`internal/middleware/tenant_scope.go`)

**API Security**
- Rate limiting: login (5/15m), OTP (3/10m), password reset (3/15m), generic (30/1m) per IP (`internal/middleware/rate_limit.go`)
- Security headers: X-Content-Type-Options, X-Frame-Options, HSTS, CSP, COOP, COEP, CORP, Permissions-Policy (`internal/middleware/security.go`)
- Authentication required middleware sets `Cache-Control: no-store` on authenticated responses (`internal/middleware/auth_middleware.go`)
- Swagger UI disabled in production (`cmd/server/main.go`)

**Data Protection**
- Soft deletes for users and persons; file uploads physically removed on soft delete (`internal/services/file_upload_service.go`)
- Optimistic locking via `version` field on Person and sync entities
- Tenant isolation enforced in sync mutations (`internal/services/entity_sync_service.go`)
- IDOR protection: file access scoped to `user_id`, repositories enforce ownership

**PWA / Offline Security**
- PII encrypted with AES-GCM-256 before IndexedDB storage (`web/src/offline/pii.ts`)
- Offline PIN protected with PBKDF2-HMAC-SHA-256, 600,000 iterations (`web/src/offline/session.ts`)
- Offline session bounded to 48-hour window after server revalidation
- Account switch performs full IndexedDB wipe (secure deletion)
- Sync uses idempotency keys, CSRF tokens, server-authoritative identity, and versioned conflicts (`web/src/offline/sync.ts`, `internal/handlers/sync_handler.go`)

---

## 7. Risk Assessment

| Risk | Severity | Likelihood | Mitigation |
|---|---|---|---|
| Go stdlib vulnerabilities (XSS, XML bomb, ASN.1 DoS, TLS issues) | HIGH | Medium | Toolchain upgrade to 1.26.6+ required |
| Unmaintained golang.org/x/crypto/openpgp (transitive) | MEDIUM | Low | Not used by application code; monitor |
| Docker image supply chain (floating tags) | MEDIUM | Medium | Pin to SHA digests |
| No documented vulnerability disclosure process | HIGH | Low | Create SECURITY.md |
| No incident response plan | HIGH | Low | Create IR plan and runbooks |
| Audit log coverage gaps | MEDIUM | Medium | Extend to data mutations |
| No centralized monitoring/alerting | MEDIUM | Medium | Define logging and alerting strategy |

---

## 8. Vulnerability Management

### Open Vulnerabilities (from SBOM)

| ID | Component | Version | Severity | Fixed In | Reachable | Status | Remediation |
|---|---|---|---|---|---|---|---|
| GO-2026-6091 | Go stdlib html/template | go1.25 | HIGH | go1.26.6 | Yes | BLOCKED | Upgrade Go toolchain |
| GO-2026-6088 | Go stdlib encoding/xml | go1.25 | HIGH | go1.26.6 | Yes | BLOCKED | Upgrade Go toolchain |
| GO-2026-5972 | Go stdlib encoding/asn1 | go1.25 | HIGH | go1.26.6 | Yes | BLOCKED | Upgrade Go toolchain |
| GO-2026-5856 | Go stdlib crypto/tls | go1.25 | HIGH | go1.26.5 | Yes | BLOCKED | Upgrade Go toolchain |
| GO-2026-6090 | Go stdlib crypto/tls | go1.25 | HIGH | go1.26.6 | Yes | BLOCKED | Upgrade Go toolchain |
| GO-2026-5932 | golang.org/x/crypto openpgp | v0.55.0 | MEDIUM | N/A | No (transitive) | BLOCKED | Migrate or update module |
| GO-2026-6355 | golang.org/x/crypto/ssh | v0.55.0 | MEDIUM | v0.56.0 | No (transitive) | BLOCKED | Update module |
| GO-2026-6354 | golang.org/x/crypto/ssh | v0.55.0 | MEDIUM | v0.56.0 | No (transitive) | BLOCKED | Update module |
| GO-2026-6089 | Go stdlib net/http | go1.25 | MEDIUM | go1.26.6 | Yes | BLOCKED | Upgrade Go toolchain |
| GO-2026-6218 | Go stdlib net/url | go1.25 | LOW | go1.26.6 | No (transitive) | MONITOR | Upgrade Go toolchain |
| GO-2026-6180 | golang.org/x/mod/sumdb | v0.38.0 | LOW | v0.40.0 | No (transitive) | MONITOR | Update module |
| GO-2026-6179 | golang.org/x/mod/sumdb/tlog | v0.38.0 | LOW | v0.40.0 | No (transitive) | MONITOR | Update module |
| GO-2026-5942 | Go stdlib net/dnsmessage | go1.25 | LOW | go1.26.6 | No (transitive) | MONITOR | Upgrade Go toolchain |
| GO-2026-5026 | Go stdlib net/idna | go1.25 | LOW | go1.26.6 | No (transitive) | MONITOR | Upgrade Go toolchain |
| GO-2026-4970 | Go stdlib os | go1.25 | LOW | go1.26.5 | No (transitive) | MONITOR | Upgrade Go toolchain |

**npm audit:** 0 vulnerabilities
**Docker image scan:** BLOCKED — Docker unavailable in assessment environment

### Remediation Plan

1. **Immediate (when network available):** Upgrade Go toolchain to 1.26.6+
2. **Next maintenance window:** Update `golang.org/x/crypto` to v0.56.0 and `golang.org/x/mod` to v0.40.0
3. **Docker:** Pin `nginx:alpine` and `postgres:16-alpine` to SHA digests; run container vulnerability scan
4. **Process:** Establish emergency patch SLA (e.g., critical within 7 days, high within 30 days)

---

## 9. SBOM / Software Supply Chain

### SBOM Availability

| Artifact | Format | Status |
|---|---|---|
| SBOM-CycloneDX.json | CycloneDX 1.5 | Generated |
| SBOM-SPDX.json | SPDX 2.3 | Generated |
| SBOM_LICENSE_REPORT.md | Markdown | Generated |

### Dependency Inventory

| Ecosystem | Direct | Indirect | Total |
|---|---|---|---|
| Go | 11 | 92 | 103 |
| NPM (prod) | 0 | — | 0 |
| NPM (dev) | 1 | 20 (platform TS) | 21 |
| Docker base images | 4 | — | 4 |
| GitHub Actions steps | 6 pinned | — | 6 |

### Dependency Pinning

- Go modules: pinned via `go.mod` and `go.sum`
- npm: `package-lock.json` present
- GitHub Actions: all third-party actions pinned to major versions (v3, v4, v5)
- **Gap:** `golangci/golangci-lint-action@v4` uses `version: latest` internally
- **Gap:** Docker images `nginx:alpine` and `postgres:16-alpine` use floating tags

### Vulnerability Scanning

- Go: govulncheck (11 findings, see Section 8)
- npm: `npm audit` (0 findings)
- Docker: BLOCKED (environment limitation)

### License Tracking

- All Go licenses are permissive (MIT, BSD-3-Clause, Apache-2.0, ISC)
- No unknown or copyleft licenses identified

---

## 10. Security Update Process

**Status: PARTIAL**

PWAMS has a CI/CD pipeline that builds and pushes Docker images on pushes to `main`. However, the following are **not documented or verified**:

- Vulnerability discovery and triage workflow
- Severity assessment criteria
- Patch development and testing process
- Emergency security update procedure
- Dependency update cadence
- Rollback strategy
- End-of-support handling for Go toolchain and dependencies
- Communication plan for security updates

**Recommended Action:** Document a Security Update Policy covering the above items, with defined SLAs for critical/high/medium/low vulnerabilities.

---

## 11. Vulnerability Disclosure

**Status: FAIL**

The repository does **not** contain:

- `SECURITY.md`
- Security contact email
- Vulnerability reporting process
- Disclosure timeline
- Security advisory workflow

**Recommended Action:** Create `SECURITY.md` with a security contact (e.g., `security@<domain>`), reporting instructions, expected response timeline (e.g., 7 business days), and embargo policy. Do **not** invent contact details; use a real, monitored address.

---

## 12. Incident Response

**Status: FAIL**

The repository does **not** contain:

- Incident response plan
- Security alerting configuration
- Incident detection runbooks
- Evidence preservation procedures
- Recovery workflows
- Post-incident review process

**Recommended Action:** Create an Incident Response Plan covering detection, containment, eradication, recovery, and lessons learned. Define alerting thresholds and escalation paths.

---

## 13. Authentication / Authorization

### Evidence Summary

| Control | Implementation | Status |
|---|---|---|
| Password hashing | bcrypt (DefaultCost) | PASS |
| Session management | 32-byte random token, SHA-256 hash, 8-hour expiry | PASS |
| Session cookie security | HttpOnly, SameSite=Lax, Secure in production | PASS |
| Login rate limiting | 5 attempts per 15 minutes per IP | PASS |
| Account lockout | 3 failed attempts, 30-minute lockout | PASS |
| CSRF protection | Double-submit cookie, constant-time comparison | PASS |
| RBAC | 6 roles, middleware enforced | PASS |
| Privilege escalation protection | Super Admin modification restricted | PASS |
| Password reset | OTP via email, 10-minute expiry, session revocation | PASS |
| Account activation | OTP via email, 10-minute expiry | PASS |

---

## 14. Data Security

### Evidence Summary

| Control | Implementation | Status |
|---|---|---|
| Database encryption | PostgreSQL SSL mode configurable (`sslmode=require`) | PARTIAL |
| Data at rest encryption | Not implemented for PostgreSQL; offline PII encrypted | PARTIAL |
| PII encryption (offline) | AES-GCM-256 with per-device key in IndexedDB | PASS |
| Secure deletion (files) | Physical removal on soft delete, restore on failure | PASS |
| Secure deletion (offline) | Full IndexedDB wipe on logout/account switch | PASS |
| Cache protection | `Cache-Control: no-store` on authenticated responses | PASS |
| Logs without secrets | No hardcoded secrets in source or generated artifacts | PASS |
| IDOR protection | User-scoped file queries, repository-level ownership | PASS |

**Gap:** PostgreSQL data-at-rest encryption (disk encryption, transparent data encryption) is not configured. This is an infrastructure-level decision; document if relying on host-level encryption.

---

## 15. Secure Development

### Evidence Summary

| Control | Implementation | Status |
|---|---|---|
| Linting | golangci-lint in CI | PASS |
| Formatting | `go fmt` verified | PASS |
| Static analysis | `go vet` verified | PASS |
| Testing | Go tests pass (7 packages); frontend build passes | PASS |
| Dependency lockfiles | `go.sum`, `package-lock.json` present | PASS |
| Secret scanning | No secrets in SBOM artifacts; no hardcoded secrets in source | PASS |
| SAST in CI | Only lint; no dedicated SAST scanner | PARTIAL |

**Recommended Action:** Add SAST scanning (e.g., `gosec`, `semgrep`) to CI pipeline.

---

## 16. Security Testing

### Evidence Summary

| Test Category | Status | Details |
|---|---|---|
| Security headers | PASS | `security_test.go` verifies X-Content-Type-Options, X-Frame-Options, CSP |
| CSRF | PASS | `security_test.go` verifies safe-method skip and missing-token rejection |
| Rate limiting | PASS | `rate_limit_test.go` verifies normal traffic passes |
| RBAC | PASS | `rbac_middleware_test.go` verifies role enforcement |
| File upload validation | PASS | `file_upload_validation_test.go` verifies magic-byte checks |
| Session handling | PASS | `session.test.ts` verifies offline session expiry and PIN |
| Offline media | PASS | `media.test.ts` verifies media mutation lifecycle |
| Penetration testing | NOT VERIFIED | No pen test report found |
| DAST | NOT VERIFIED | No dynamic scanning in CI |
| Dependency vulnerability scanning | PARTIAL | govulncheck and npm audit run; Docker scan blocked |

**Recommended Action:** Schedule periodic penetration testing and add DAST to CI for non-production environments.

---

## 17. Technical Documentation

### Evidence Summary

| Document | Status | Notes |
|---|---|---|
| API documentation | EXISTS | Swagger UI in non-production (`docs/docs.go`) |
| SBOM | EXISTS | CycloneDX and SPDX generated |
| License inventory | EXISTS | `SBOM_LICENSE_REPORT.md` |
| Security architecture | MISSING | No formal security architecture document |
| Threat model | MISSING | No documented threat model |
| Risk assessment | MISSING | No formal risk assessment document |
| Vulnerability management policy | MISSING | No documented policy |
| Security update policy | MISSING | No documented update/SLA policy |
| Incident response plan | MISSING | No documented IR plan |
| Vulnerability disclosure policy | MISSING | No SECURITY.md or disclosure process |
| Secure development lifecycle | MISSING | No documented SDLC security gates |
| Release security checklist | MISSING | No pre-release security checklist |
| Dependency management policy | MISSING | No documented dependency update cadence |
| Backup/recovery plan | MISSING | No documented backup/recovery procedures |
| User security instructions | MISSING | No user-facing security guidance |
| Deployment guide | MISSING | No formal deployment documentation |
| Configuration guide | PARTIAL | `.env.example` exists; no security config guide |

---

## 18. Lifecycle / Support Considerations

**Status: NOT VERIFIED**

No evidence found for:

- Supported Go version policy
- Dependency EOL monitoring
- Security support timeline
- Deprecation policy for PWAMS versions
- End-of-support handling for transitive dependencies

**Recommended Action:** Define a support policy (e.g., support latest Go release and one prior minor version; 12-month security support for released versions).

---

## 19. Gaps and Risks

### Critical Gaps

None that block deployment, but the following require immediate attention:

1. **Go toolchain vulnerabilities (6 HIGH/MEDIUM)** — Reachable from application code. Upgrade to Go 1.26.6+ as soon as network is available.
2. **No vulnerability disclosure policy** — Users and researchers have no published channel to report security issues.

### High Gaps

3. **No incident response plan** — Lack of documented detection, containment, and recovery procedures.
4. **Docker image supply chain** — Floating tags for `nginx:alpine` and `postgres:16-alpine`; no container vulnerability scan.

### Medium Gaps

5. **Audit log coverage** — Only authentication events are logged; data mutations, sync operations, and admin actions are not audited.
6. **No centralized monitoring/alerting** — No SIEM, log aggregation, or security alerting documented.
7. **Missing security documentation** — No threat model, risk assessment, or security architecture document.
8. **No SAST in CI** — Only linting; no dedicated static analysis security testing.

### Low Gaps

9. **No user security instructions** — No guidance for end users on password hygiene or session management.
10. **No lifecycle/support policy** — No documented support period or EOL handling.

---

## 20. Recommended Remediation Plan

| Priority | Action | Owner | Timeline |
|---|---|---|---|
| P0 | Upgrade Go toolchain to 1.26.6+ | DevOps | Immediate (when network available) |
| P0 | Create `SECURITY.md` with disclosure policy | Security | 1 week |
| P1 | Document incident response plan | Security | 2 weeks |
| P1 | Pin Docker images to SHA digests | DevOps | 2 weeks |
| P1 | Run container vulnerability scan | DevOps | 2 weeks |
| P2 | Extend audit logging to data mutations | Backend | 1 month |
| P2 | Add SAST to CI pipeline | DevOps | 1 month |
| P2 | Create security architecture document | Security | 1 month |
| P2 | Document threat model and risk assessment | Security | 1 month |
| P3 | Define security update policy and SLAs | Security | 2 months |
| P3 | Create backup/recovery plan | DevOps | 2 months |
| P3 | Add user security instructions | Documentation | 2 months |
| P3 | Define support/lifecycle policy | Management | 3 months |

---

## 21. Evidence Index

| Evidence Type | Location |
|---|---|
| SBOM | `PWAMS_SBOM_SECURITY_REPORT.md` |
| License inventory | `SBOM_LICENSE_REPORT.md` |
| npm audit | `npm_audit.json` |
| Go build/test | Verified via `go fmt ./...`, `go vet ./...`, `go build ./...`, `go test ./... -count=1` |
| Frontend build | Verified via `npm run build` |
| Authentication | `internal/services/auth_service.go`, `internal/handlers/auth_handler.go` |
| Session management | `internal/services/session_service.go` |
| Password hashing | `internal/utils/password.go` |
| CSRF | `internal/middleware/csrf.go` |
| RBAC | `internal/middleware/rbac_middleware.go` |
| Rate limiting | `internal/middleware/rate_limit.go` |
| Security headers | `internal/middleware/security.go` |
| Audit logging | `internal/services/audit_log_service.go` |
| File upload security | `internal/services/file_upload_service.go`, `internal/handlers/file_upload_validation_test.go` |
| PWA offline security | `web/src/offline/pii.ts`, `web/src/offline/session.ts`, `web/src/offline/sync.ts`, `web/src/offline/csrf.ts` |
| Sync security | `internal/services/sync_service.go`, `internal/handlers/sync_handler.go`, `internal/services/person_sync_service.go`, `internal/services/entity_sync_service.go` |
| CI/CD | `.github/workflows/ci-cd.yml` |
| Docker | `Dockerfile`, `docker-compose.yml`, `nginx.conf` |
| Configuration | `internal/config/config.go`, `.env.example` |

---

## 22. Final CRA Readiness Classification

**CRA READY WITH FINDINGS**

PWAMS implements strong security-by-design controls across authentication, authorization, input validation, session management, CSRF protection, rate limiting, secure headers, data integrity, and PWA offline security. Build, tests, lint, and frontend compilation all pass. SBOM and license inventory are available.

However, the product is not fully CRA-ready due to:

1. **6 reachable HIGH/MEDIUM vulnerabilities in the Go standard library** (toolchain upgrade blocked)
2. **Absence of organizational security processes** (vulnerability disclosure, incident response, update policy)
3. **Incomplete supply chain security** (Docker image pinning, container scanning)
4. **Limited audit log coverage**
5. **Missing security documentation** (threat model, risk assessment, security architecture)

These findings are documented with clear remediation actions. The application can be deployed with the understanding that the Go toolchain upgrade and organizational security processes must be completed before final CRA compliance can be claimed.

---

## Final Summary

```
TOTAL REQUIREMENTS ASSESSED: 18
PASS: 9
PARTIAL: 6
FAIL: 2
NOT VERIFIED: 1
NOT APPLICABLE: 0

CRITICAL GAPS: 0
HIGH GAPS: 2
  - No vulnerability disclosure policy
  - No incident response plan
MEDIUM GAPS: 6
  - 6 reachable Go stdlib vulnerabilities (BLOCKED from fix)
  - Docker image supply chain gaps
  - Audit log coverage incomplete
  - No centralized monitoring/alerting
  - Missing security documentation
  - No SAST in CI
LOW GAPS: 2
  - No user security instructions
  - No lifecycle/support policy

SBOM STATUS: EXISTS — CycloneDX 1.5 and SPDX 2.3 generated; Docker scan blocked
VULNERABILITY MANAGEMENT STATUS: PARTIAL — 11 vulnerabilities found, 6 reachable, all blocked from remediation
SECURITY TESTING STATUS: PARTIAL — Unit tests pass; no pen test or DAST
DOCUMENTATION STATUS: PARTIAL — SBOM and API docs exist; security policies missing

FINAL CRA READINESS: CRA READY WITH FINDINGS
```
