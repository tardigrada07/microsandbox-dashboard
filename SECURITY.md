# Security Policy

## Reporting a vulnerability

Please do not open a public issue. Report privately through
[GitHub's advisory form](https://github.com/tardigrada07/microsandbox-dashboard/security/advisories/new)
(Security tab, "Report a vulnerability"). Include the version or commit, your OS and steps to reproduce.

You should get a reply within a week. Fixes go into the latest release only.

## Scope

The dashboard can create and delete microVMs and mount host directories into them, so it is meant to be
reachable only by the user who started it. In scope:

- Reaching the dashboard from another machine, or binding a non-loopback address.
- Using the API without the per-run token, or from another origin (CSRF, DNS rebinding).
- Injecting script into the UI through sandbox names, env, paths or other data.
- Input that bypasses server-side validation (names, image refs, env keys, paths, resource limits).

Out of scope: anything that needs the token or local access as your user already, and issues in
Microsandbox itself (report those to [microsandbox](https://github.com/superradcompany/microsandbox)).

## Verifying releases

Release binaries come with a `SHA256SUMS` file and a signed build provenance attestation:

```sh
sha256sum -c SHA256SUMS --ignore-missing
gh attestation verify msbdash-linux-amd64 --repo tardigrada07/microsandbox-dashboard
```
