//go:generate solc -o . --abi Router.sol
//go:generate abigen --abi Router.abi --pkg router --type Router --out Router.go

package router
