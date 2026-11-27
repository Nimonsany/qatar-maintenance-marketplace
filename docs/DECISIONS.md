# QATAR MAINTENANCE MARKETPLACE — DECISION REGISTER

## Approved Decisions

### DEC-001 — Development Workflow
Status: APPROVED
Decision: Build the project in Chat Mode, slowly and phase by phase, with one verified task at a time.

### DEC-002 — Project Continuity
Status: APPROVED
Decision: Maintain PROJECT_MASTER.md, PROJECT_STATE.md, DECISIONS.md, CHANGELOG.md and HANDOFF.md as persistent project records.

### DEC-003 — Existing Project Safety
Status: APPROVED
Decision: Keep this project isolated. Do not modify other repositories, databases, services or project configurations.

### DEC-004 — Approval Gates
Status: APPROVED
Decision: Require owner approval before starting a new development phase or performing potentially destructive operations.

### DEC-005 — AI Collaboration
Status: APPROVED
Decision: ChatGPT provides step-by-step architecture, implementation guidance and review support. Codex may provide independent technical review. Do not rely on NVIDIA Nemotron as the primary architect.

## Proposed Technical Direction — NOT YET APPROVED
- Go modular-monolith backend
- PostgreSQL and PostGIS
- Next.js and TypeScript for web
- Flutter for mobile apps
- Cloud provider undecided

## External Dependencies — NOT APPROVED
- Qatar legal eligibility and permitted maintenance-work model
- Customer payment collection and PSP settlement arrangements
- Data protection, insurance, consumer rights and payroll boundaries

## Change Control
Never silently overwrite an approved decision. Record a new decision with the reason and owner approval before changing it.



## 2026-11-27 — Governance Reconciliation Record

### Purpose
Reconcile verified backend development with the historical
decision register without modifying previously approved decisions.

### Existing Approved Decisions
DEC-001 through DEC-005 remain approved and unchanged.

No previous approval is revoked, replaced or silently amended.

### Verified Technical Checkpoint
- Repository: Nimonsany/qatar-maintenance-marketplace
- Completed backend milestone: DEV-004E
- Git commit:
  117c1bfed966acabb4b8e1da35314995823c2c83
- Local main and origin/main synchronized at last verification.
- Backend regression and race tests: PASS.
- Code and Git safety review: PASS.

### Historical Phase 0.1 Governance Gap
- Phase 0.0 completion remains historically verified.
- The existing five continuity documents do not record
  formal owner approval for Phase 0.1.
- Phase 0.1 approval reconciliation remains unresolved.
- Subsequent backend commits do not, by themselves,
  establish formal Phase 0.1 approval or completion.
- Do not retroactively invent or infer an approval.

### DEV-004F Planning
Status: PLAN MODE ONLY

Proposed topic:
Backend Lifecycle Reliability Hardening.

Proposed work:
- Active-request graceful shutdown tests
- Shutdown timeout verification
- Forced-close fallback tests
- Deterministic test synchronization
- Preservation of existing API and security behavior

These are proposed technical requirements, not approved
BUILD instructions.

DEV-004F BUILD: NOT AUTHORIZED.

### Documentation Reconciliation Authorization
Status: OWNER AUTHORIZED

Scope:
- docs/PROJECT_STATE.md
- docs/HANDOFF.md
- docs/CHANGELOG.md
- docs/DECISIONS.md
- docs/PROJECT_MASTER.md

Conditions:
- Manual VS Code editing
- One document at a time
- Verify each step before proceeding
- Preserve historical decisions and records
- No backend modifications
- No database or infrastructure modifications
- No Git staging, commit or push
- No DEV-004F BUILD

### Pending Owner Decisions
- Reconcile historical Phase 0.1 approval evidence.
- Review and approve final DEV-004F technical scope.
- Provide separate BUILD authorization if appropriate.
- Provide separate Git authorization for documentation
  staging, commit and push.

### Change Control
This section records verified status and current restrictions.
It does not create or modify DEC-001 through DEC-005.

Any new binding architectural or business decision
requires explicit owner approval and a separate record.


## 2026-11-27 — DEV-008 Decision Register Status Addendum

This status addendum does NOT create, revoke or change any binding decision.
DEC-001, DEC-002, DEC-003, DEC-004 and DEC-005 remain APPROVED, unchanged.
The earlier DEV-004F PLAN-only and past documentation authorization entries
are preserved as historical records, not today's permission state.

### Verified Progress (Evidence, Not a New Approval)
- DEV-004F completed at a4b222e1aa01d4507c8d524861239c7d75769d8e.
- DEV-005 completed at 4a0be0e8b198f9ec0f495df444c8fff6f588f475.
- DEV-006 completed at 9b13814667e29009aed1150bcb5b1d3f4ad9b740.
- DEV-007 completed at db8f2e16773c8f5bb3b3efbf49adcb3393548778.
- Latest DEV-007 test and remote SHA verification: PASS.

### Unresolved Approvals and Controls
- No formal Phase 0.1 owner-approval record appears in the historical
  five continuity documents; the governance gap remains UNRESOLVED.
- DEV-008 authorizes documentation-only append reconciliation.
- Git staging, commits and push require separate approval.
- Existing proposed technical direction is not automatically ratified.
- No approval for live public marketplace endpoints, authentication
  integrations, database migrations, payment/PSP operations, cloud
  infrastructure or legally restricted Qatar services.
- Any new binding architecture/business decision needs an explicit
  owner approval and separately recorded decision number.
