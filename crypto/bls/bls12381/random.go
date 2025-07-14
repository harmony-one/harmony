package bls12381

import (
	"crypto/rand"
	"math/big"
)

func (g *G1) CreateRandomG1() *PointG1 {
	k, err := rand.Int(rand.Reader, q)
	if err != nil {
		panic(err)
	}
	return g.MulScalar(&PointG1{}, g.One(), k)
}

func (g *G2) CreateRandomG2() *PointG2 {
	k, err := rand.Int(rand.Reader, q)
	if err != nil {
		panic(err)
	}
	return g.MulScalar(&PointG2{}, g.One(), k)
}

func CreateRandomScalar(max *big.Int) *big.Int {
	a, _ := rand.Int(rand.Reader, max)
	return a
}
