# auth

OAuth compliant server to issue JWTs for other services.

Dependancies:

- web-ingress (TLS termination)
- postgres (see [db.md](db.md))

## Env variables to set

```bash
export AUTH_PORT=80
export AUTH_DSN=postgresql://some_dsn
export AUTH_PRIVATE_KEY=pem_encoded_ecdsa_key
export AUTH_PUBLIC_KEY=pem_encoded_ecdsa_key
export AUTH_JWT_ISSUER=your.domain.org
export AUTH_WEBAUTHN_RPID=your.domain.org
export AUTH_WEBAUTHN_RPORIGIN=https://your.domain.org
export AUTH_WEBAUTHN_RPDISPLAYNAME=Auth Service
```

## Endpoints

Any case where the response is ommited, the response should be a 204 (No content). Any case where the request is ommited, you are not expected to provide a body.

### `POST /create_client`

```
Auth:  Required
Scope: auth:create_client
```

Create an OAuth client so a new service can integrate with this auth provider.

#### Request

```json
{
	"name": "my_client",
	"redirect_uris": [
		"http://localhost:3000",
		"https://my-app.org"
	]
}
```

- `redirect_uris` can be `null`, or an array of URL strings. If providing URLs they must include the scheme but no path.

#### Response

```json
{
	"client_id": "client_000000C3gv5uIKETVF8zXNuBQOCT2"
}
```

### `POST /create_invite_code`

```
Auth:  Required
Scope: auth:create_invite_code
```

Create an invite code to use on the [`GET /register`](#browser-only-endpoints) endpoint. Allows a new user to sign up for an account. You may only give scopes you possess.

#### Request

```json
{
	"scopes": "auth:login short:upload",
	"code": "AABBCC",
	"expires_at": "2021-02-01T00:00:00Z"
}
```

- `code` can be `null`, the service will generate a code for you.
- `expires_at` can be `null` which would mean there is no expiry for this code.

#### Response

```json
{
	"code": "AABBCC",
	"expires_at": "2021-02-01T00:00:00Z"
}
```

### `POST /delete_key`

```
Auth:  Required
Scope: -
```

Allows a user to delete their own U2F keys. If the requester has the `auth:delete_keys` scope, they can delete other peoples keys.

#### Request

```json
{
	"user_id": "user_000000C3gvy8kjxBDTNxNRMIkx518",
	"key_id": "u2fcredential_000000C3gvzngY5VXAqFeqlpsXZmp"
}
```

### `POST /list_u2f_keys`

```
Auth:  Required
Scope: -
```

Allows a user to list their own U2F keys. If the requester has the `auth:list_keys` scope, they can list other users U2F keys, not just their own.

#### Request

```json
{
	"user_id": "user_000000C3gvy8kjxBDTNxNRMIkx518",
	"include_unsigned": false
}
```

#### Response

```js
[
	{
		"id": "u2fcredential_000000C3gvzngY5VXAqFeqlpsXZmq",
		"name": "15 557 235",
		"signed_at": "2020-03-01T00:00:00Z",
		"public_key": "some_long_string"
	},
	...more,
]
```

- `signed_at` can be `null` for unsigned keys.

### `POST /whoami`

```
Auth:  Required
Scope: -
```

In case a client forgets their associated `user_id` or `username`, this endpoint can get them from the JWT for you. A client should not attempt to read or parse the JWT themselves.

#### Response

```json
{
	"user_id": "user_000000C3gwazN3teFfrekQnP1KIHi",
	"username": "John Smith"
}
```

### `POST /token`

```
Auth:  Anonymous
Scope: -
```

Used as part of the OAuth flow to redeem an authorization code and turn it into an access token.

#### Request

```json
{
	"client_id": "client_000000C3gx7kZQ0umyKwQFisVD1OS",
	"grant_type": "code",
	"redirect_uri": "https://my-app.org",
	"code": "some-long-code",
	"code_verifier": "pkce-confirmation-string"
}
```

- `redirect_uri` can be `null` if your app has no redirect URIs set up.

## Browser only endpoints

`GET /authorize`

Used to start the OAuth flow, send users here with the usual suspets, a `client_id`, some scopes, a PKCE hash etc as URL params.

`GET /register`

For users to sign up for the first time. Convert an invitation code into a username and setup their first U2F key.

`GET /u2f_manage`

For users to manage their U2F keys with a friendly UI.

`POST /authorize_confirm`

Used in the background of the OAuth flow for WebAuthn.

`POST /authorize_prompt`

Used in the background of the OAuth flow for WebAuthn.

`POST /create_key_confirm`

WebAuthn endpoints for adding new keys.

`POST /create_key_prompt`

WebAuthn endpoints for adding new keys.

`POST /register_confirm`

WebAuthn endpoints for registering for the first time.

`POST /register_prompt`

WebAuthn endpoints for registering for the first time.

`POST /sign_key_confirm`

WebAuthn endpoints for signing a new key with an existing one.

`POST /sign_key_prompt`

WebAuthn endpoints for signing a new key with an existing one.
