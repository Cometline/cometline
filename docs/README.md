# Docs

This directory holds contributor-facing guides that sit on top of `README.md`,
`ARCHITECTURE.md`, and `ARCHITECTURE_GUIDE.md`.

Read these in order when onboarding to the repo:

1. `../README.md`
2. `../ARCHITECTURE.md`
3. `../ARCHITECTURE_GUIDE.md`
4. `learning/00-README.md` — progressive learning series (start here for a guided tour)
5. `MODULE_GUIDE.md`
6. `SETTINGS_AND_PERSISTENCE.md`
7. `FRONTEND_DESIGN_SYSTEM.md`

Guides in this folder:

- `learning/` - progressive onboarding series (nutshell → architecture → modules → recipes)
- `MODULE_GUIDE.md` - module ownership, boundaries, and change checklists
- `SETTINGS_AND_PERSISTENCE.md` - settings modal rules, dirty-state guardrails, and cross-layer save flow
- `FRONTEND_DESIGN_SYSTEM.md` - current visual system, styling rules, and known frontend consistency gaps
- `postmortems/` - incident and root-cause writeups

Historical postmortems preserve the route names and component names involved in the incident. If a postmortem mentions an older API path, prefer the living architecture docs above for the current contract.
