# Deployment Manifests Review Guidelines

Applies to: `{deploy,manifests}/**`

The following files are the only manually managed files under the
`deploy/` directory:

- `deploy/deploy.sh`
- `deploy/Dockerfile*` (all Dockerfiles under `deploy/`)
- `deploy/service_account.yaml`
- `deploy/webhooks.yaml`

Only the manually managed files above need review. All the rest,
including files within sub-directories, are auto-generated and
should be ignored during review
