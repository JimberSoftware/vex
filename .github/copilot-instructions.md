---
applyTo: "**"
---

# Project general coding standards

## Comments and Documentation

- No comments inside functions except for:
  - Complex algorithms requiring explanation
  - TODO/FIXME markers
  - Regulatory/compliance requirements
- Do not create README files unless asked to do so
- Never add agent as co author in commit messages, even if the agent contributed to the code. The agent is a tool, not a person, and should not be credited as an author.
- never commit spec files

## format and lint

- use `golangci-lint`
