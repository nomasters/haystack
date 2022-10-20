# haystack

a simple and ephemeral content addressed key/value store.

Design goals:
- no auth, the submitter declares the hash of the content and submits the content. if the hash of the content on the server matches the submitted hash, the content is accepted and any requestor with the hash can request the content
- ephemeral - key/value is short lived, no keys will live longer than a window (TBD) and this is defined by the server. Content can be resubmitted to reset the counter
- submitted value should be indistinguishable from random data. Values should not be discernable by the server. The server will do statistical analysis on the payloads qualify a minimum threshold. This analysis is also known to the client and the client should test before submitting.

key - blake2(256) hash of the content
value - bytes (should be )

goals:
- easy to use client
- fast
- simple

```
key      | value
---------|----------
32 bytes | 160 bytes
```

This is large enough for the value to contain something like:

|  nonce   |     encrypted payload        |
|          |------------------------------|  
|          | next key    | padded message |
|----------|-------------|----------------|
| 24 bytes | 32 bytes    |  bytes   104   |


The response includes a hash, timestamp, a mac of the data, and a private key signed of all data behind it.
Optionally, you can add a preshared key to the mac and a private key to the nacl_sign.
```submitted_hash||timestamp||blake2_256_mac(submitted_hash||timestamp)||nacl_sign(submitted_hash||timestamp||blake2_256_mac)```

If a preshared key is not inlucded, the mac is simply of the hash + timestamp, and the nacl_sign bits are always included even if a private or pub key are not present, if they are not present, the server generates a preshared key and signs the payload, even though the client doesn't have a way to verify. This gives us a consistent payload reguardless of implementation  
