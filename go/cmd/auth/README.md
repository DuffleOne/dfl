# auth

A CLI tool to handle authentication.

## Installation

`export DFL_AUTH_URL=https://auth.dfl.mn`

`go install ./cmd/auth/...`

## Usage

### `login`

`auth login`

Login via OAuth. It'll try to open a browser window. Make sure you check the state against the one given in the CLI. You can optionally provide `-s [scope]` too if you want a custom scope.

### `manage`

`auth manage`

Tries to take you to the U2F management page. Just a URL to follow.

### `show-access-token`

`auth show-access-token`

Or `sat` for short. Show the access token and some information about it. Can pipe this with `pbcopy` to copy the access token to your clipboard quickly.

`auth sat | pbcopy`
