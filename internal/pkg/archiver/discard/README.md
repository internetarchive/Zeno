# Discard Hook

## Must close `resp.Body` before copying to `io.Discard`

```go
discarded, discardReason = client.DiscardHook(resp)

if discarded {
    resp.Body.Close()              // First, close the body, to stop downloading data anymore.
    io.Copy(io.Discard, resp.Body) // Then, consume the buffer.
}
```

Otherwise, although the record will not be written to the WARC file, io.Copy will keep downloading, wasting bandwidth and gowarc's temporary space.

## Discard Hook functions works at both GOWARC and Zeno layers

This is due to the way `gowarc` works, as it uses `io.Pipe` to MITM HTTP connections, which somewhat isolates the upper layer from the lower layer's `net.Conn`.

However, `gowarc` does pass the lower-level `warc.CustomConnection(net.Conn)` to the upper layer through `req.Context`'s `wrappedConn` channel.

So you might think that we can just call `discarded, discardReason = client.DiscardHook(resp)` once at the upper layer (i.e., in `Zeno`), and if `discarded = true`, we can close the lower-level connection with `warc.CustomConnection.CloseWithError(discardReason)`.

We wouldn't need to call `DiscardHook` again in `gowarc`, right?

But that's not the case.

When we call `client.Do(req)` in `Zeno`, it will first call `gowarc.RoundTrip(req)` to handle the request (it's just a warpper of `http.RoundTrip`), which will read the request and response from the wrapped `net.Conn` (`warc.CustomConnection`).

```go
resp, err = client.Do(req)
```

And if the response's `Content-Length` is 0, at this point, the RoundTrip has already read all data from the connection and does not need to wait for the caller to read `resp.Body` (just returns empty), so it will close the connection normally (for responses with `Content-Length != 0` or streaming payload, it will keep the HTTP connection open and wait for the caller to read `resp.Body` or close it). At this point, `gowarc` has also read the `req` and `resp` pairs from the wrapped `net.Conn` (`warc.CustomConnection`) without any hindrance and has written the records to the WARC file normally.


Then `client.Do(req)` returns, and we have the `resp` at the Zeno layer, and we lost the opportunity to call `warc.CustomConnection.CloseWithError(discardReason)` to cancel/discard the WARC records.

Therefore, we need to call `DiscardHook` at both the lower level in `gowarc` and the upper level in `Zeno`. For those 0 `Content-Length` responses... :(

Until we find a better way to handle this. :)
