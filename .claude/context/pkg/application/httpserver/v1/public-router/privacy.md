# Privacy policy page (`get-privacy-policy.go`)

| Route | Access | Success |
|---|---|---|
| `GET /privacy` | public | 200 HTML page |

The public privacy policy that Google Play (Privacy policy and Data deletion fields) links to: `https://bloom-health-service.vercel.app/privacy`. It's an HTML page, not part of the JSON API or the contract.

## Rules
- The text is `assets/privacy-policy.md`, a copy of the app's `docs/privacy-policy.md`, which the app generates from `src/lib/privacyPolicy.ts`. Change the policy in the app first, then copy the file here. Nothing checks the two repos match.
- `renderPolicyPage` handles only what that file uses: `#` and `##` headings, `- ` bullets, a line wrapped in `_`, paragraphs, and email addresses (linked as `mailto:`). Everything is HTML-escaped first.
- The page overrides two global headers: CSP adds `style-src 'unsafe-inline'` (still no scripts), and `Cache-Control` is `public, max-age=3600`.

## Gotchas
- Registered in `pkg/application/httpserver/router.go`, outside `/v1`.
- The global per-IP rate limit applies.
