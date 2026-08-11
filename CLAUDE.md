# Working agreements

## Audits (security, code review, dependency, etc.)

When asked to audit, review, or scan anything in this repo:

1. **Audit first, don't fix as you go.** Investigate and record findings only — do not edit files during the audit pass.
2. **Produce a summary as a Markdown file** listing what needs to change (what/where/why, and severity/impact if relevant).
3. **Stop and confirm** with the user before making any of the changes in that summary.
4. **Only after confirmation**, make the changes.

If delegating the checklist to another AI assistant/subagent, state this explicitly in its instructions: audit only, produce a written summary, then stop and ask before editing anything.
