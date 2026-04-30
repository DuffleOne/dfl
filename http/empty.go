package http

// Empty is the sentinel used as Req or Resp for handlers with no body.
// As a Resp it produces a 204 No Content response. As a Req no decoding
// happens and any request body is ignored.
type Empty struct{}
