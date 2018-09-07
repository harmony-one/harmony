go-pow
======

`go-pow` is a simple Go package to add (asymmetric) *Proof of Work* to your service.

To create a Proof-of-Work request (with difficulty 5), use `pow.NewRequest`:

```go
req := pow.NewRequest(5, someRandomNonce)
```

This returns a string like `sha2bday-5-c29tZSByYW5kb20gbm9uY2U`,
which can be passed on to the client.
The client fulfils the proof of work by running `pow.Fulfil`:

```go
proof, _ := pow.Fulfil(req, []byte("some bound data"))
```

The client returns the proof (in this case  `AAAAAAAAAAMAAAAAAAAADgAAAAAAAAAb`)
to the server, which can check it is indeed a valid proof of work, by running:


```	go
ok, _ := pow.Check(req, proof, []byte("some bound data"))
```

Notes
-----
1. There should be at least sufficient randomness in either the `nonce` passed to
   `NewRequest` or the `data` passed to `Fulfil` and `Check`.
   Thus it is fine to use the same bound `data` for every client, if every client
   get a different `nonce` in its proof-of-work request.
   It is also fine to use the same `nonce` in the proof-of-work request,
   if every client is (by the encapsulating protocol) forced to use
   different bound `data`.
2. The work to fulfil a request scales exponentially in the difficulty parameter.
   The work to check it proof is correct remains constant:

   ```
   Check on Difficulty=5  	  500000	      2544 ns/op
   Check on Difficulty=10 	  500000	      2561 ns/op
   Check on Difficulty=15 	  500000	      2549 ns/op
   Check on Difficulty=20 	  500000	      2525 ns/op
   Fulfil on Difficulty=5  	  100000	     15725 ns/op
   Fulfil on Difficulty=10 	   30000	     46808 ns/op
   Fulfil on Difficulty=15 	    2000	    955606 ns/op
   Fulfil on Difficulty=20 	     200	   6887722 ns/op
   ```

To do
-----

 - Support for [equihash](https://www.cryptolux.org/index.php/Equihash) would be nice.
 - Port to Python, Java, Javascript, ...
 - Parallelize.

