# drop-tracker
A self-hosted tracker that pings me when the artists and producers I follow drop something new, including the guest verses and producer credits Spotify or the artist hide.

## Local setup

After cloning, run:

```
make hooks
```

This installs a pre-commit hook that scans staged changes for secrets with
[gitleaks](https://github.com/gitleaks/gitleaks) before a commit can land. The pinned
version lives in `.pre-commit-config.yaml`.
