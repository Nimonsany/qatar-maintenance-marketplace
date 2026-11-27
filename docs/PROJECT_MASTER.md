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
