# Authentication

yafu supports three authentication modes. Pick one based on what's
already deployed in your environment:

| Mode | When to use | Trust model |
|------|-------------|-------------|
| `anonymous` | Local dev, evaluating yafu | Every request gets a synthetic identity. **Not for production.** |
| `header` | A reverse proxy (oauth2-proxy, Pomerium, ingress auth-snippet, ...) already terminates auth and injects identity headers | yafu trusts `X-Forwarded-User` / `-Email` / `-Groups`. The proxy MUST strip these from the public Internet. |
| `oidc` | yafu speaks OIDC directly to your IdP | Authorization-code-with-PKCE flow. Native session cookie, no extra proxy needed. |

This guide focuses on **OIDC mode** with worked recipes for three
common IdPs. Header mode is documented at the end.

## Common OIDC setup

Regardless of IdP, you need to:

1. Register an OAuth client in the IdP and obtain `client_id` +
   `client_secret`.
2. Set the redirect URI to **exactly** `https://<your-yafu-host>/auth/callback`.
3. Make sure the issued ID token includes a groups claim (or
   equivalent) — yafu uses it for RBAC matching.
4. Store `client_secret` (and optionally a cookie-signing secret) in
   a Kubernetes Secret.

```sh
kubectl -n yafu-system create secret generic yafu-oidc \
  --from-literal=client_secret=<from-IdP> \
  --from-literal=cookie_secret=$(openssl rand -hex 32)
```

Then in your Helm values:

```yaml
auth:
  mode: oidc
  oidc:
    issuer: https://<idp-host>
    clientId: yafu
    redirectURL: https://yafu.example.com/auth/callback
    scopes: openid,email,profile,groups
    groupsClaim: groups
    secureCookie: true
    secretRef:
      name: yafu-oidc
      clientSecretKey: client_secret
      cookieSecretKey: cookie_secret
```

The chart will mount the Secret at `/etc/yafu/oidc/` and pass the
matching `--oidc-client-secret-file` / `--oidc-cookie-secret-file`
flags. The cookie-signing secret is optional — if omitted, yafu
generates a random one at startup (cookies become invalid on pod
restart, which is fine for single-replica deployments).

## RBAC policy

Once authenticated, requests go through the policy engine. With no
`--rbac-file` set, every authenticated user has full access — yafu
logs a WARN at startup. Provide a policy file via Helm:

```yaml
authzPolicy:
  policy: |
    defaultAction: deny
    rules:
      - subjects: ["group:platform-admins"]
        verbs: ["*"]
        clusters: ["*"]
        action: allow

      - subjects: ["group:sre-oncall"]
        verbs: ["get", "reconcile"]
        clusters: ["prod-*"]
        action: allow

      - subjects: ["group:dev-team"]
        verbs: ["get"]
        clusters: ["dev-*", "staging"]
        action: allow
```

The full schema (subject syntax, verb list, cluster globs) is
documented in [`deploy/sample-rbac-policy.yaml`](../deploy/sample-rbac-policy.yaml).

## Recipe: Dex

[Dex] is a lightweight OIDC provider often used as the federation
layer in front of LDAP, GitHub, Google, etc.

In your Dex config:

```yaml
staticClients:
  - id: yafu
    name: yafu
    secret: <generated-secret>
    redirectURIs:
      - https://yafu.example.com/auth/callback
```

In yafu values:

```yaml
auth:
  mode: oidc
  oidc:
    issuer: https://dex.example.com
    clientId: yafu
    redirectURL: https://yafu.example.com/auth/callback
    scopes: openid,email,profile,groups
    groupsClaim: groups
    secretRef:
      name: yafu-oidc
```

Dex emits the `groups` claim by default when an upstream connector
(LDAP/GitHub/Google) provides it. Test the round-trip with
`/api/v1/whoami` — it should return the expected `groups` array.

## Recipe: Keycloak

In Keycloak, create a confidential client:

- Client type: OpenID Connect
- Client ID: `yafu`
- Client authentication: On
- Standard flow: enabled
- Valid redirect URIs: `https://yafu.example.com/auth/callback`

Add a **Group Membership** mapper to the client's dedicated scope
so the `groups` claim ships in the ID token. By default Keycloak
prefixes group names with `/` — strip it with the mapper's
"Full group path" toggle.

In yafu values:

```yaml
auth:
  mode: oidc
  oidc:
    issuer: https://keycloak.example.com/realms/<realm>
    clientId: yafu
    redirectURL: https://yafu.example.com/auth/callback
    scopes: openid,email,profile,groups
    groupsClaim: groups
    secretRef:
      name: yafu-oidc
```

## Recipe: GitHub via oauth2-proxy (header mode)

GitHub does not speak OIDC, so use [oauth2-proxy] in front of yafu
and run yafu in **header mode**:

```yaml
# yafu values
auth:
  mode: header

# oauth2-proxy values (excerpt)
config:
  configFile: |
    provider = "github"
    client_id = "<from-GitHub-OAuth-app>"
    client_secret = "<...>"
    redirect_url = "https://yafu.example.com/oauth2/callback"
    upstreams = ["http://yafu.yafu-system.svc:80"]
    email_domains = ["acme.example"]
    pass_authorization_header = true
    pass_user_headers = true
    set_xauthrequest = true
    github_org = "acme"
    github_team = "platform,sre-oncall"
```

Configure your ingress so that `/oauth2/*` is routed to oauth2-proxy
and everything else is proxied through it to yafu. oauth2-proxy
injects `X-Forwarded-User`, `X-Forwarded-Email`, and
`X-Forwarded-Groups`. yafu reads those.

**Critical**: the ingress must strip those headers from any incoming
request. Otherwise an attacker can set them directly. yafu has no
way to distinguish forged headers from legitimate ones — that's the
trade-off of header mode.

## Verifying the round-trip

Whichever mode you pick, after login hit:

```sh
curl -s https://yafu.example.com/api/v1/whoami | jq .
```

You should see your subject, email, and groups. If `groups` is empty
when you expect values, your IdP isn't including the claim in the ID
token — fix the IdP configuration (Dex connector, Keycloak mapper,
oauth2-proxy `pass_user_headers`).

[Dex]: https://dexidp.io/
[oauth2-proxy]: https://oauth2-proxy.github.io/oauth2-proxy/
