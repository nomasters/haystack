# haystack

an ephemeral, quiet and tiny content addressed key/value store.

Design goals:
- no auth. the submitter declares the hash of the content and submits the content. if the hash of the content on the server matches the submitted hash, the content is accepted and any requestor with the hash can request the content
- ephemeral. key/value is short lived, no keys will live longer than a window (TBD) and this is defined by the server. Content can be resubmitted to reset the counter
- consistent payload and response to make implementation details indistinguishable to an adversary  
- quiet. No network traffic happens that isn't absolutely needed.



goals:
- easy to use client and server
- fast
- simple

### Needle: the Haystack message

A message in haystack is called a Needle. A needle includes a 32 bytes sha256 hash of a 160 byte payload.
All payloads are a fixed length. Both the client and server can verify the hash of the payload by verifying a 192 byte needle.

```
hash     | payload
---------|----------
32 bytes | 160 bytes
```

The message size is small, but designed to allow _single UDP packet_ message transmission. This is meant to be light weight and efficient.

While it is small, it is large enough to "chain" messages together. Such patterns must be configured client-side, but a hypothetical payload with an encrypted message

This is large enough for the value to contain something like:

|  nonce   |     encrypted payload        |
|          |------------------------------|  
|          | next key    | padded message |
|----------|-------------|----------------|
| 24 bytes | 32 bytes    |  bytes   104   |



### Reads and Writes

A Haystack server accepts two and only two byte length requests 32 bytes and 192 bytes.

#### Read Requests

A 32 byte request is a read request. It implies that the requestor would like the stored value of the Needle. If the Haystack server has a Needle that matches the request hash, it will respond with the full Needle in the response.

#### Write Requests

A write request my be 192 bytes. The server will verify that these bytes are a valid Needles (that the final 160 bytes sha256 hash match the first 32 bytes hash included in the payload). If this Needle is valid, it is stored. The server provides no response for this operation. If a client wants to confirm that a write was completed successfully, it should submit a read request to confirm.



If a preshared key is not included, the mac is simply of the hash + timestamp, and the nacl_sign bits are always included even if a private or pub key are not present, if they are not present, the server generates a preshared key and signs the payload, even though the client doesn't have a way to verify. This gives us a consistent payload regardless of implementation.