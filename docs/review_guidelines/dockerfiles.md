# Dockerfile Review Guidelines

Applies to: `**/Dockerfile*`

- Review container security best practices
- Check for minimal base images
- [Order instructions from least to most frequently changing](https://docs.docker.com/build/cache/optimize/)
  to maximize layer cache reuse between versions
- Verify proper handling of secrets and environment variables
- Review multi-stage build optimizations
