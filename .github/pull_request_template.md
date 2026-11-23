## Summary

-

## Test plan

- [ ] `make test` / CI green
- [ ] Manual check if UI or deploy path changed

## Security checklist

- [ ] No secrets, private keys, or `.env` files committed
- [ ] User/runtime containers do not mount the Docker socket
- [ ] User/runtime containers stay off the Postgres/Redis network
- [ ] New endpoints enforce auth and ownership
- [ ] Logs/API responses do not print tokens or credentials
- [ ] Timeouts / resource limits still apply to long-running work
- [ ] Docs updated if trust boundaries or controls changed (`docs/security.md`)
