# haystack

a simple and ephemeral content addressed key/value store.

Design goals:
- no auth, the submitter declares the hash of the content and submits the content. if the hash of the content on the server matches the submitted hash, the content is accepted and any requestor with the hash can request the content
- ephemeral - key/value is short lived, no keys will live longer than a window (TBD) and this is defined by the server. Content can be resubmitted to reset the counter
- submitted value should be indestingishable from random data. Values should desernable by the server. The server will do statistical analysis on the payloads qualify a minimum threshold. This analysis is also known to the client and the client should test before submitting.

key - blake3(256) hash of the content
value - bytes (should be )

goals:
- easy to use client
- fast
- simple

```
key      | value
---------|----------
32 bytes | 448 bytes
```

This is large enough for the value to contain something like:

|  nonce   |     encrypted payload        |
|          |------------------------------|  
|          | next key    | padded message |
|----------|-------------|----------------|
| 24 bytes | 32 bytes    | 392 bytes      |
