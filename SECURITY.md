# Security Policy

## Supported Versions

Only the latest release receives security fixes. No long-term support branches exist yet.

| Version | Supported |
|---------|-----------|
| latest  | yes       |
| older   | no        |

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Report by email to **cold.harp5900@fastmail.com** with:

- A description of the vulnerability and its potential impact
- Steps to reproduce or a proof-of-concept
- Any suggested mitigations

You can expect an acknowledgment within 72 hours.

Since Purser is self-hosted, you may also open a pull request with a fix directly — you don't need to wait for a maintainer patch. If you go that route, keep the PR description vague until it's merged, then we can document the details in the release notes.

## Scope

Areas most relevant to security in Purser:

- HTTP API (authentication, authorization, input validation)
- File path handling and on-disk operations
- Integration with external services (metadata sources, download clients, indexers)
- Configuration handling (secrets and credentials in config)
