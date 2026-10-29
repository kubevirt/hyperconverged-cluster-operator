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
- Do not edit `docs/api.md` or `docs/api-v1beta1.md` directly; update the
  underlying Go doc comments on API structs and run `make generate-doc`
- In user documentation (e.g. `docs/cluster-configuration.md`), focus on
  user-facing behavior rather than internal implementation details
