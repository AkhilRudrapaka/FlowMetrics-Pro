# Configuration

With Docker Compose you only edit `.env` (copied from [.env.example](../.env.example), which documents every value inline). Compose passes the relevant variables to the containers. The table lists the **backend's** environment variables as the Go service reads them; the server fails fast with a clear message if a required one is missing or weak.

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `MONGO_URI` | yes | | MongoDB connection string. Compose builds it from `MONGO_ROOT_USER` / `MONGO_ROOT_PASSWORD` |
| `JWT_SECRET` | yes | | HS256 signing key, **at least 32 characters** |
| `MONGO_DB` | no | `flowmetrics` | Database name |
| `APP_ENV` | no | `development` | `production` enables Secure cookies and JSON logs, and **rejects secrets containing `change-me`** |
| `PORT` | no | `8080` | Listen port |
| `ACCESS_TOKEN_TTL` | no | `15m` | Access-token lifetime (Go duration) |
| `REFRESH_TOKEN_TTL` | no | `168h` | Session lifetime |
| `COOKIE_SECURE` | no | `true` in production, else `false` | Mark the refresh cookie `Secure` (needs HTTPS) |
| `BCRYPT_COST` | no | `12` | bcrypt work factor (4–31) |
| `AUTH_RATE_LIMIT_PER_MIN` | no | `20` | Per-IP limit on login/register; `0` disables |
| `CORS_ALLOWED_ORIGINS` | no | empty | Comma-separated origins. Only needed if the UI is served from a different origin than the API; the bundled nginx and Vite proxy make this unnecessary |
| `TRUSTED_PROXIES` | no | empty | CIDRs allowed to set `X-Forwarded-For`. Compose sets the private Docker ranges so the rate limiter sees real client IPs |
| `STATIC_DIR` | no | empty | When set, the API also serves the built web UI from this directory (SPA fallback, CSP, asset caching). Used by the single-image [Render deployment](deploy-render.md) (`/public`); Docker Compose leaves it unset and uses nginx instead |
| `CLIENT_IP_HEADER` | no | empty | Header that carries the real visitor IP behind a CDN, e.g. `CF-Connecting-IP` on Render. Leave empty when nothing trustworthy sets it: a client-supplied header would otherwise let anyone dodge the login rate limiter |
| `SEED_ON_START` | no | `false` | Insert demo data if the database has no users. **Never in production** |
| `SEED_PASSWORD` | no | `Password123!` | Password for the seeded accounts |

Compose-only variables: `MONGO_ROOT_USER`, `MONGO_ROOT_PASSWORD` (database root account), `WEB_PORT` (host port for the UI, default 3000), `VITE_DEMO_HINT` (build-time: show demo accounts on the login page).

## Going beyond a local demo

- Set `APP_ENV=production`, `COOKIE_SECURE=true`, a random `JWT_SECRET` (`openssl rand -hex 32`), strong database credentials, and `SEED_ON_START=false`.
- Put TLS in front (a load balancer or a TLS-terminating proxy). The refresh cookie is `Secure` only when `COOKIE_SECURE=true`.
- Back up the `mongo-data` volume; consider a MongoDB replica set if you need transactions.
