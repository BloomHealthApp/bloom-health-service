# IConnectionService (`connection-service.go`)

Profile links, friend requests and friendships ("Friends" in the app).

## Rules
- A request can only be made through a profile link code (`users.profileLinkCode`, 12 random base62 characters from `crypto/rand`). There is no lookup by email, phone or id for people who aren't friends. `ResetProfileLink` replaces the code; a clash with another user's code (unique index) is retried.
- Other users only leave this service as `response.PublicProfileResponse` (`helpers.PublicProfileResponse`). Stats (`IStatsService.ProfileStats`: current and longest streak, active habit count) are added only for friends and for the caller's own profile; lists never carry stats, so listing 200 friends doesn't compute 200 summaries.
- `Profile` and `Activity` share `friendOrSelf`: the caller or an accepted friend who is still live, otherwise `errNoProfile`.
- Unknown, reset, malformed and pending-deletion links all return the same `errNoProfile` 404 (`byLink`).
- One `dao.Connection` per pair (`pairKey`, unique). Asking someone who already asked you accepts it (mutual). A decline turns the record into a `declined` marker with `expiresAt` = now + `DeclinedRequestCooldown` (7 days); during it the requester gets 429 with `Retry-After`, and their view goes back to `relation: "none"`. The person who declined can still ask later; their request replaces the marker. An expired marker is deleted on the next request (the TTL index may not have run yet).
- `Remove`: unfriending first deletes the pair's chat (`IDirectMessageRepo.DeletePair`), so a failure leaves them friends and a retry finishes it. Either side can unfriend; only the requester can withdraw a pending request (the addressee uses `Decline`); decline markers can't be removed here, or the cooldown could be skipped.
- `List` drops users that are gone or pending deletion; `Profile` and `Accept` treat them as not found.

## Gotchas
- `setLinkCode` re-reads the user before saving because `IUserRepo.Save` replaces the whole document.
