# Documentation Review Guidelines

Applies to: `docs/**`

- New user-facing API knobs must have a user-guide-style explanation
  in `docs/cluster-configuration.md`, not just CRD field descriptions
- Explain implicit side effects (e.g., feature gates auto-enabled,
  webhooks created, additional operands deployed)
- After renaming a Markdown heading, check and update all `#anchor`
  fragment links in the same file; broken internal links are a bug
- Documentation must be understandable by cluster administrators who
  are not Go developers
