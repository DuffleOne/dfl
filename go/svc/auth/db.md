# database

## `access_tokens`

```sql
CREATE TABLE access_tokens (
    id text PRIMARY KEY,
    token text NOT NULL,
    authorization_code text NOT NULL REFERENCES authorization_codes(id) UNIQUE,
    expires_at timestamp without time zone NOT NULL
);
```

## `authorization_codes`

```sql
CREATE TABLE authorization_codes (
    id text PRIMARY KEY,
    client_id text NOT NULL REFERENCES clients(id),
    user_id text NOT NULL REFERENCES users(id),
    state text NOT NULL,
    nonce text NOT NULL,
    code_challenge_method text NOT NULL,
    code_challenge text NOT NULL,
    scope text NOT NULL,
    response_type text NOT NULL,
    created_at timestamp without time zone NOT NULL DEFAULT now(),
    expires_at timestamp without time zone NOT NULL
);

CREATE UNIQUE INDEX authorization_codes_user_id_nonce_idx ON authorization_codes(user_id text_ops,nonce text_ops);
```

## `clients`

```sql
CREATE TABLE clients (
    id text PRIMARY KEY,
    name text NOT NULL UNIQUE,
    redirect_uris text[] NOT NULL DEFAULT ARRAY[]::text[],
    created_at timestamp without time zone NOT NULL DEFAULT now()
);
```

## `invitations`

```sql
CREATE TABLE invitations (
    id text PRIMARY KEY,
    code text NOT NULL UNIQUE,
    scopes text NOT NULL,
    created_by text REFERENCES users(id),
    created_at timestamp without time zone NOT NULL,
    redeemed_by text REFERENCES users(id) UNIQUE,
    redeemed_at timestamp without time zone,
    expires_at timestamp without time zone
);
```

## `u2f_challenges`

```sql
CREATE TABLE u2f_challenges (
    id text PRIMARY KEY,
    challenge text NOT NULL,
    user_id text NOT NULL REFERENCES users(id),
    allowed_credentials_id text[] NOT NULL DEFAULT ARRAY[]::text[],
    user_verification text,
    created_at timestamp without time zone NOT NULL DEFAULT now(),
    expires_at timestamp without time zone NOT NULL
);
```

## `u2f_credentials`

```sql
CREATE TABLE u2f_credentials (
    id text PRIMARY KEY,
    key_id text NOT NULL,
    user_id text NOT NULL REFERENCES users(id),
    name text,
    public_key text NOT NULL,
    attestation_type text NOT NULL,
    authenticator jsonb NOT NULL DEFAULT '{}'::jsonb,
    u2f_challenge_id text NOT NULL REFERENCES u2f_challenges(id),
    signed_at timestamp without time zone,
    created_at timestamp without time zone NOT NULL DEFAULT now(),
    deleted_at timestamp without time zone
);
```

## `users`

```sql
CREATE TABLE users (
    id text PRIMARY KEY,
    username text NOT NULL UNIQUE,
    email text UNIQUE,
    created_at timestamp without time zone NOT NULL DEFAULT now(),
    scopes text
);
```
