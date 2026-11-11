# Design skills: where they came from

Copied by hand from each repository on 2026-11-11 (no installer scripts were run).
To update a skill, re-copy its folder from the same repository and change the commit here.

| Skill folders | Repository | Commit | Licence |
|---|---|---|---|
| frontend-design, webapp-testing | github.com/anthropics/skills | 2c5958e | Apache 2.0 |
| interface-design (+ `.claude/commands/design-review.md`, `design-deslop.md`) | github.com/Dammyjay93/interface-design | 2f9be32 | MIT |
| emil-design-eng, animate, gestures, mobile-native, apple-design, choreograph, find-animation-opportunities, review-animations, improve-animations, remove-animations, animation-performance, animation-vocabulary, break-ui, prototype, pick-ui-library | github.com/emilkowalski/skills | 715c180 | MIT |
| web-design-guidelines | github.com/vercel-labs/agent-skills | 1a2c79d | MIT |
| `docs/design/ux-checklist/` (reference docs only, not a skill) | github.com/nextlevelbuilder/ui-ux-pro-max-skill | 6ae86f6 | MIT |

## Left out on purpose

- **Taste Skill** (github.com/Leonxlnx/taste-skill): version 2.5 is for marketing websites and adds a Stop hook. Revisit for a Fuxi landing page.
- **UI/UX Pro Max as a skill**: its style database suggests looks that fight Fuxi's palette. Only its UX checklist is kept, as docs.
- From Emil's pack: `write-swift`, `animate-expo` (native iOS / React Native) and `ask-sonner` (add it if the client uses Sonner toasts).

## Notes

- `web-design-guidelines` fetches its rules from raw.githubusercontent.com on every run.
- `prototype`, `pick-ui-library` and `review-animations` only run when called by name.
