# Harmony Benchmark
[![Build Status](https://travis-ci.com/simple-rules/harmony-benchmark.svg?token=DnoYvYiTAk7pqTo9XsTi&branch=master)](https://travis-ci.com/simple-rules/harmony-benchmark)

## Golang Coding Convention

* Follow [effective_go](https://golang.org/doc/effective_go.html)
* Constant enum should follow CamelCase.
* Comments of each element in a struct is written right after the element.

## Dev Environment Setup


```
export GOPATH=$HOME/<path_of_your_choice>

mkdir -p $HOME/<path_of_your_choice>/src

cd $HOME/<path_of_your_choice>/src

git clone git@github.com:simple-rules/harmony-benchmark.git

cd harmony-benchmark

go get github.com/go-stack/stack
```
## Usage
```

./deploy.sh local_config.txt

```







