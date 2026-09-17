---
title: OIDC with Authentik
---

How to point rmfakecloud at [Authentik](https://goauthentik.io/). The full list of
variables is in the [configuration reference](../../configuration/#oidc-login-for-the-web-ui).

Three things about Authentik differ from most providers and each one fails in a
way that is hard to read backwards from the error. They are called out below.

## Provider

In the Authentik admin interface, go to **Applications** > **Providers** and
create an **OAuth2/OpenID Provider**.

| Field | Value |
|---|---|
| Client type | Confidential |
| Client ID | copy it, this is `OIDC_CLIENT_ID` |
| Client Secret | copy it, this is `OIDC_CLIENT_SECRET` |
| Redirect URIs | `https://rm.example.com/ui/api/oidc/callback`, as a strict match |
| Signing Key | any configured certificate |

Under **Advanced protocol settings**:

| Field | Value |
|---|---|
| Scopes | `openid`, `email`, `profile` |
| Subject mode | leave at the default |
| **Include claims in id_token** | **enabled** |
| Issuer mode | Per Provider (the default) |

Then create an **Application**, give it a slug, and bind the provider to it. The
slug becomes part of the issuer URL, so pick it before configuring rmfakecloud.

## Trap 1: the issuer needs its trailing slash

Authentik's issuer for a per-provider application is:

```
https://authentik.example.com/application/o/<application-slug>/
```

with a trailing slash. rmfakecloud strips a trailing slash before asking for the
discovery document, so discovery succeeds either way, but it then compares the
issuer the provider reports against the string you configured. Without the
slash those differ, and the log carries:

```
oidc: issuer did not match the issuer returned by provider
```

The server still starts and everything else keeps working, but the OIDC login
button answers `503` until the value is corrected and rmfakecloud is restarted.

So `OIDC_PROVIDER_URL` must end in `/`.

## Trap 2: claims have to be put in the ID token

rmfakecloud reads the userid and the admin claim from the ID token, never from
the userinfo endpoint. Authentik only puts scope-mapping claims in the ID token
when **Include claims in id_token** is enabled on the provider. With it off,
discovery works, the login round trip works, and then the login is rejected for
having no userid, because the ID token carried none.

## Trap 3: groups come from `profile`, not from a `groups` scope

Authentik's default `profile` scope mapping already returns `groups`, along with
`preferred_username`, `name`, `given_name`, `family_name` and `nickname`. There
is no separate `groups` scope to request, so leave `OIDC_EXTRA_SCOPES` unset.
Asking for a `groups` scope that has no mapping behind it gets you nothing.

## Environment

```env
OIDC_PROVIDER_URL=https://authentik.example.com/application/o/rmfakecloud/
OIDC_CLIENT_ID=<client id from the provider>
OIDC_CLIENT_SECRET=<client secret from the provider>
OIDC_REDIRECT_URL=https://rm.example.com/ui/api/oidc/callback
RM_HTTPS_COOKIE=true
OIDC_ADMIN_CLAIM=groups
OIDC_ADMIN_CLAIM_VALUE=rmfakecloud-admins
OIDC_DISPLAY_NAME=Login with Authentik
```

Replace `authentik.example.com` with your Authentik host, `rmfakecloud` with your
application slug, and `rm.example.com` with the host rmfakecloud is reached at.
Create a group named `rmfakecloud-admins` in Authentik and put your administrators
in it, or point `OIDC_ADMIN_CLAIM_VALUE` at a group you already have.

Password login stays available alongside this. Set `OIDC_DISABLE_LOCAL_LOGIN=true`
only once you have confirmed the round trip works, and remember that from then on
a broken provider also means no way to mint a tablet enrolment code.

## About `email_verified`

Authentik's default `email` scope mapping returns `email_verified` hard-coded to
`false`. It is not reporting that the address is unverified, it simply does not
track that state.

This does not matter with the default `OIDC_USERID_CLAIM=preferred_username`,
because the verification requirement only applies when the user id is itself an
email address. It does matter if you set `OIDC_USERID_CLAIM=email`, or if
`preferred_username` is empty for some user and the fallback to email kicks in.
In those cases the login is refused with `email not verified`.

Two ways out, in order of preference:

1. Keep `preferred_username` as the user id. Authentik always populates it from
   the username
2. Write a custom scope mapping that returns `"email_verified": True`, and use it
   in place of the default `email` mapping, if your Authentik really does verify
   addresses

`OIDC_ALLOW_UNVERIFIED_EMAIL=true` also works and is the blunt option. It turns
the check off for every user, so only reach for it if you accept that anyone who
can set an email address in Authentik can claim the matching rmfakecloud account.

## Checking it

With the server running and OIDC configured, this should report the provider and
the label:

```bash
curl -s https://rm.example.com/ui/api/oidc/info
```

```json
{"displayName":"Login with Authentik","enabled":true,"localLoginEnabled":true}
```

If `enabled` is false, one of the four required variables is missing. The server
logs which at startup.

`enabled` being true only means the four variables are set. Whether the provider
actually answers shows up in the startup log, and in the response to the login
button: a `503` with `the identity provider is not reachable` means the variables
are in place and the round trip is not. rmfakecloud retries discovery on each
login attempt, so once Authentik answers, logins start working without a
restart.

## Troubleshooting

### `missing state cookie` at the callback

The browser came back from Authentik with a code, but without the cookie
rmfakecloud set when it started the flow. The flow is stateless on the server:
state, nonce and the PKCE verifier live entirely in three short-lived cookies,
so losing them means the callback cannot be tied to the request that began it.

Work out which side is losing them, in this order.

Does the server send them? This should print a `302`, a `Location` at your
provider, and three `Set-Cookie` lines:

```bash
curl -s -o /dev/null -D - https://your-domain.com/ui/api/oidc/login \
  | grep -iE 'HTTP/|set-cookie|location'
```

Does the server receive them? This starts a flow, keeps the cookies, and presents
them back with a deliberately invalid code:

```bash
rm -f /tmp/oidcjar
curl -s -c /tmp/oidcjar -o /dev/null https://your-domain.com/ui/api/oidc/login
STATE=$(grep oidc_state /tmp/oidcjar | awk '{print $NF}')
curl -s -b /tmp/oidcjar "https://your-domain.com/ui/api/oidc/callback?code=bogus&state=$STATE"
```

`token exchange failed` is the answer you want: the state check passed and the
flow only stopped on the invalid code. `missing state cookie` here means a
reverse proxy is dropping the request cookie.

If both commands behave and a browser still fails, it is that browser. Try a
private window: if the login works there, a cookie-blocking extension or a
cookie-clearing rule in the normal profile is the cause. Allow both your
rmfakecloud and provider domains in it.

### The flow cookies expire after five minutes

Clicking the login button and then leaving the provider's page open for longer
than that produces the same `missing state cookie`. Start again rather than
reloading the callback URL.

### `userid claim not found or empty`

The ID token carried no `preferred_username`. On Authentik this is almost always
**Include claims in id_token** being off on the provider.

### `email not verified`

The user id resolved to an email address, and Authentik's default `email` scope
mapping reports `email_verified` as `false`. See the section above.
