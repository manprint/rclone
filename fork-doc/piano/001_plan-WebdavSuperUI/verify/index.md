# WebdavSuperUI — Audit register

## Historical reports
| ID | File | Date | Original verdict | Original counts | Auditor |
|----|------|------|------------------|-----------------|---------|
| V001 | [verify_001_2026-10-21.md](verify_001_2026-10-21.md) | 2026-10-21 | PASS WITH FINDINGS | 0 BLOCKER, 0 MAJOR, 3 MINOR (2 corretti nello stato durante l'audit) | Opus (agent-1) |

Verdicts: PASS, PASS WITH FINDINGS, FAIL, INCOMPLETE. Historical counts/verdicts
are not recomputed as findings are fixed; current unresolved findings are below.

## Findings
| ID | Severity | Title | Current status | Closed by / date | Evidence |
|----|----------|-------|----------------|------------------|----------|
| V001-F1 | MINOR | I download senza credenziali non trattengono la VFS (`Config.HoldVFS` inutilizzato) | FIXED | V001-C1 (unit:V001-C1:1), 2026-10-21 | `u.hold(` in `token.go`/`public.go`; `TestPreAuthHoldsVFS` PASS + mutazione; STATE §7 |
| V001-F2 | MINOR | Commit post-chiusura `56897460b` fuori da un'unità registrata | FIXED | riconciliazione di stato V001, 2026-10-21 | riga ad-hoc in STATE §4 |
| V001-F3 | MINOR | Intestazione di STATE.md ferma a "chiusura P2" | FIXED | riconciliazione di stato V001, 2026-10-21 | STATE riga 3 |

Statuses: OPEN, FIXED, ACCEPTED (user decision), OBSOLETE (reason recorded).
Never silently remove a finding. Update the source report's resolution too.

## Open blockers
none
