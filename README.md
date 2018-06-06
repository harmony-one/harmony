# Harmony Benchmark

## Usage


## References

https://github.com/lightningnetwork/lnd

## New Plan:


## Done
* Week of 5/21: https://docs.google.com/document/d/1u-8C9MiEUYPA7QC1Ekg7Cg-yL09ArZIxxyUtCUF9Wp0/edit

## TODO

* implement block of transactions, instead of individual generators
* instead of _consume_ used leader.send function
* understand when the program terminates and why we don't end up writing all the txns
* simplify code, if it might be a overkill
* Read [go concurrency](https://gist.github.com/rushilgupta/228dfdf379121cb9426d5e90d34c5b96) to setup transaction channel and pour those txns (randomString) into a queue from where the leader node can read it in chunks.


## Readings

* [fanout-example](https://play.golang.org/p/jwdtDXVHJk)
* [multiple-gorountines-listening-on-one-channel](https://stackoverflow.com/questions/15715605/multiple-goroutines-listening-on-one-channel/15721380#15721380)
* [go concurrency](https://gist.github.com/rushilgupta/228dfdf379121cb9426d5e90d34c5b96)
