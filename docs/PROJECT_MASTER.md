# QATAR MAINTENANCE HYBRID MARKETPLACE

## Project Objective
Build a Qatar-first hybrid maintenance marketplace connecting customers with legally eligible individual maintenance workers and maintenance companies.

## Platforms
- Customer Website: Next.js
- Customer Mobile App: Flutter
- Worker Mobile App: Flutter
- Maintenance Company Portal: Next.js
- Admin Dashboard: Web

## Core Services
Plumbing, electrical, painting, mechanical, civil maintenance, welding/iron works, and other approved maintenance categories.

## Core Features
- Instant, hourly, daily, multi-day and fixed-price service booking
- Customer job requests with photos and location
- Quotations and customer approvals
- Multi-worker scheduling and replacement
- Worker and company verification with legal eligibility checks
- Nearby worker matching and category notifications
- Emergency requests, attendance, disputes and ratings
- Cash payments and legally approved PSP options
- English and Arabic customer support; Nepali, Hindi, Bengali and Urdu worker support

## Technical Direction (Proposed)
- Backend: Go modular monolith
- Database: PostgreSQL + PostGIS
- Web: Next.js and TypeScript
- Mobile: Flutter
- Hosting and cloud: Undecided

## Development Rules
1. Work in Chat Mode, one step at a time.
2. Verify each step before continuing.
3. Owner approval is required before starting a new phase.
4. Do not modify other projects, databases or services.
5. Update PROJECT_STATE.md and HANDOFF.md at milestones.
6. Use Git and backups once separately approved.
7. Do not activate legally restricted work or payment flows without required Qatar legal/PSP verification.
8. Never claim tests passed unless actually executed.

## Current Status
Phase 0.0 — COMPLETE. Phase 0.1 — AWAITING OWNER APPROVAL.
No application coding, migrations or deployment has started.



## 2026-11-27 — Verified Technical Progress Addendum

### Purpose
Record current verified technical progress without changing
the original project objective, proposed architecture,
business scope or approved development rules.

### Verified Backend Progress
- Backend foundation implemented through DEV-004E.
- Latest completed milestone: DEV-004E.
- Repository: Nimonsany/qatar-maintenance-marketplace
- Verified Git commit:
  117c1bfed966acabb4b8e1da35314995823c2c83
- Branch: main
- Local and origin/main synchronized at last verification.
- Backend module: qatar-maintenance/backend
- Go version declared in backend/go.mod: 1.26

### Implemented Backend Foundation
- Local GET /health endpoint
- Graceful HTTP server shutdown
- Structured JSON API error responses
- Cryptographically generated request IDs
- Baseline HTTP security headers
- Centralized validated application configuration
- Structured JSON logging with sensitive-key redaction
- Reusable HTTP server and lifecycle components
- Application bootstrap integration
- Occupied-port and SIGTERM integration tests

### DEV-004E Verification Evidence
- Full backend regression tests: PASS.
- Go race detector: PASS.
- go vet: PASS.
- Formatting and Git whitespace checks: PASS.
- Code and Git safety review: PASS.
- Local Git commit: COMPLETE.
- GitHub remote synchronization: VERIFIED.

### Current Development Direction
- DEV-004F: PLAN MODE.
- Proposed topic: Backend Lifecycle Reliability Hardening.
- Proposed focus: active-request shutdown, timeout behavior,
  forced-close fallback and deterministic lifecycle tests.
- DEV-004F BUILD: NOT AUTHORIZED.

### Governance Status
- Phase 0.0: Historical completion verified.
- Existing continuity documents do not record formal
  Phase 0.1 owner approval.
- Phase 0.1 governance reconciliation remains unresolved.
- Later technical progress does not automatically prove
  formal Phase 0.1 approval or completion.
- Preserve DEC-001 through DEC-005 and all historical records.

### Implementation Boundaries
- Existing business features remain project requirements,
  not claims of implemented marketplace functionality.
- Database migrations: Not started.
- Cloud deployment: Not started.
- Payment or PSP activation: Not authorized.
- Qatar legal eligibility and operating model verification:
  Still required before legally restricted operations.
- Hosting and cloud provider: Undecided.

### Current Authorized Work
Documentation-only reconciliation of the five project
Markdown documents, one file at a time.

Not authorized:
- Backend code changes
- Database migrations
- Infrastructure modifications
- Git staging, commit or push
- DEV-004F BUILD

### Change Control
This addendum preserves the original project master record.
Any binding change to architecture, business scope or
approved development rules requires separate owner approval.


## 2026-11-27 — DEV-008 Current Technical Status (Superseding Addendum)

This section is the CURRENT checkpoint. Earlier dated statements about
"DEV-004F PLAN MODE", "no application coding", or previous authorization
were accurate only for their respective historical snapshots.
They are not current operating instructions.

### Verified Milestones and Git Checkpoints
- DEV-004F: HTTP lifecycle reliability hardening — COMPLETE;
  commit a4b222e1aa01d4507c8d524861239c7d75769d8e.
- DEV-005: isolated marketplace domain and booking state machine — COMPLETE;
  commit 4a0be0e8b198f9ec0f495df444c8fff6f588f475.
- DEV-006: application services, repository and trusted-identity contracts — COMPLETE;
  commit 9b13814667e29009aed1150bcb5b1d3f4ad9b740.
- DEV-007: isolated, unregistered HTTP API adapter and contract tests — COMPLETE;
  commit db8f2e16773c8f5bb3b3efbf49adcb3393548778.
- At the DEV-007 checkpoint: local main and GitHub origin/main SHAs matched;
  working tree was clean; backend regression, race tests, vet and formatting PASS.
- Backend module declares Go 1.26.

### Current Operational Boundaries
- Implemented API adapter is NOT registered in the application router.
- No live public marketplace booking or quotation endpoints are activated.
- No production authentication, verified worker eligibility, database
  migrations, PSP/payment activation, cloud deployment or infrastructure work.
- PostgreSQL/PostGIS and technology proposals are not approved solely
  because these Go foundations were implemented.
- Phase 0.0 historical completion: verified; Phase 0.1 formal approval
  evidence remains absent from the five original continuity records.
  Phase 0.1 governance reconciliation: UNRESOLVED.
- DEC-001 through DEC-005 remain unchanged and approved.

### Current Authorization
- DEV-008 documentation-only reconciliation of the five named Markdown
  continuity files is approved; preserve all historical sections.
- No backend code modifications or unrelated file modifications.
- Git stage, commit and GitHub push require separate explicit authorization.
- No new runtime feature or DEV-009 BUILD authorization is implied.
