Consensus package includes the Harmony BFT consensus protocol code.

For now, the current implementation is still in experimental phase because it's coded using the two-round Schnorr-based multi-signature similar to [Byzcoin's design](https://www.usenix.org/system/files/conference/usenixsecurity16/sec16_paper_kokoris-kogias.pdf).

We will migrate to BLS-based multi-signature following Harmony's new [consensus protocol design](https://talk.harmony.one/t/bls-based-practical-bft-consensus/131).
