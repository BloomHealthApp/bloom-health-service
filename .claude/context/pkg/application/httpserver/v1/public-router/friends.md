# Friends and profile links

Handlers: `get-profile-link.go`, `reset-profile-link.go`, `get-profile-by-link.go`, `send-connection-request.go`, `list-connections.go`, `accept-connection.go`, `decline-connection.go`, `delete-connection.go`, `get-profile.go`, `get-profile-activity.go`, plus the public `get-public-profile-page.go` and `get-assetlinks.go` (registered outside `/v1` in `httpserver/router.go`). Service: `IConnectionService` ([connection-service.md](../../../../domain/service/connection-service.md)).

- `GET /v1/profiles/by-link/:code` and `GET /v1/profiles/:id` share a prefix; gin allows the static `by-link` segment next to the `:id` param.
- `by-link` and `POST /connections` sit behind `middleware.LimitFriends()` (per user, after auth), so link codes can't be guessed. `/u/:code` has `LimitProfilePage()` (per IP).
- `/u/:code` is reachable without a token, so it shows only `bo.PublicCard` (first name and initial), sends `X-Robots-Tag: noindex` and `Cache-Control: no-store`, and answers unknown, reset, malformed and deleted links with one identical 404 page.
- `assetlinks.json` lists `com.bloomhealthapp.app` with `ANDROID_CERT_SHA256S`; with none configured it serves `[]`, so Android opens `/u/` links in the browser page instead of the app.
