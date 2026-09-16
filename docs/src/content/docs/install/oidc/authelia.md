---
title: OIDC with Authelia
---

How to point rmfakecloud at [Authelia](https://www.authelia.com/). The full list
of variables is in the [configuration reference](../../configuration/#oidc-login-for-the-web-ui).

## Client secret

Generate a secret and its hash:

```bash
authelia crypto hash generate argon2 --random --random.length 64 --random.charset alphanumeric
```

The command prints both. The hash goes in Authelia's config, the plaintext goes in
`OIDC_CLIENT_SECRET`.

## Claims policy

Authelia 4.38 and later leave `groups` out of the ID token even when the `groups`
scope is requested. rmfakecloud reads the admin claim from the ID token, so
without a claims policy naming `groups`, `OIDC_ADMIN_CLAIM=groups` never matches.

Add this to `identity_providers.oidc`, not inside `clients`:

```yaml
identity_providers:
  oidc:
    claims_policies:
      with_groups:
        id_token:
          - email
          - email_verified
          - groups
          - preferred_username
          - name
```

## Client

```yaml
identity_providers:
  oidc:
    clients:
      - client_id: 'rmfakecloud'
        client_name: 'rmfakecloud'
        client_secret: '$argon2id$v=19$...'  # the hash from above
        public: false
        authorization_policy: 'one_factor'   # or 'two_factor'
        consent_mode: implicit
        claims_policy: 'with_groups'
        redirect_uris:
          - 'https://rm.example.com/ui/api/oidc/callback'
        scopes:
          - 'openid'
          - 'email'
          - 'profile'
          - 'groups'
        userinfo_signed_response_alg: 'none'
        token_endpoint_auth_method: 'client_secret_basic'
```

## Admin group

Create a group in your Authelia user database and put the users who should
administer rmfakecloud in it. The name below is an example; it only has to match
`OIDC_ADMIN_CLAIM_VALUE`.

## rmfakecloud environment

```env
OIDC_PROVIDER_URL=https://auth.example.com
OIDC_CLIENT_ID=rmfakecloud
OIDC_CLIENT_SECRET=<the plaintext secret from above>
OIDC_REDIRECT_URL=https://rm.example.com/ui/api/oidc/callback
RM_HTTPS_COOKIE=true
OIDC_ADMIN_CLAIM=groups
OIDC_ADMIN_CLAIM_VALUE=rmfakecloud-admins
OIDC_EXTRA_SCOPES=groups
OIDC_DISPLAY_NAME=Login with Authelia
```

Replace `auth.example.com` with your Authelia host and `rm.example.com` with the
host rmfakecloud is reached at.

Password login stays available alongside this. Set
`OIDC_DISABLE_LOCAL_LOGIN=true` once you have confirmed the flow works, and
remember that at that point a broken provider also means no way to mint a tablet
enrolment code.
