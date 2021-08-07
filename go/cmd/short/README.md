# short-cli

A CLI tool that allows you to upload files to the above server!

## Install

Install it into your PATH

`go install ./cmd/short/...`

## Env variables to set

```
export DFL_SHORT_URL=https://dfl.mn
export DFL_AUTH_URL=https://auth.dfl.mn
```

### Upload a file

Upload a single file:

`short signed-upload {file}`

`short u my-file.png`

It will attempt to automatically put the URL in your clipboard too!

### Shorten URL

Shorten a URL

`short shorten {url}`

`short s https://google.com/?query=something-long`

See other params above.

### Copy a URL

When given a long URL leading to an image, it'll attempt to download the file and reupload it to the short server.

`short copy {url}`

`short c https://avatars1.githubusercontent.com/u/1222340?s=460&v=4`

### Set it as NSFW

Set the file as NSFW so a NSFW primer appears before the content. The user must agree before they continue.

`short nsfw {url or hash}`

`short n ddA`

### Add a shortcut

Add a shortcut to the resource, so there is an easy way to access the resource

`short add-shortcut {url or hash} {shortcut}`

`short asc https://dfl.mn/aaA yolo`

### Remove a shortcut

Remove a shortcut from the resource

`short remove-shortcut {url or hash} {shortcut}`

`short rsc aaA yolo`

### Screenshot

macOS only so far, this one handles the whole screenshot process for you. Bind this to a shortcut on your mac so you can quickly take a snippet of a program and the link appears in your clipboard

`short screenshot`
