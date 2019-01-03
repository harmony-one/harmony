# Harmony Benchmark
[![Build Status](https://travis-ci.com/harmony-one/harmony.svg?token=DnoYvYiTAk7pqTo9XsTi&branch=master)](https://travis-ci.com/harmony-one/harmony)
<a href='https://github.com/jpoles1/gopherbadger' target='_blank'>![gopherbadger-tag-do-not-edit](https://img.shields.io/badge/Go%20Coverage-39%25-brightgreen.svg?longCache=true&style=flat)</a>


## Coding Guidelines

* In general, we should follow [effective_go](https://golang.org/doc/effective_go.html)
* Code must adhere to the official [Go formatting guidelines](https://golang.org/doc/effective_go.html#formatting) (i.e. uses [gofmt](https://golang.org/cmd/gofmt/)).
* Code must be documented adhering to the official Go [commentary](https://golang.org/doc/effective_go.html#commentary) guidelines.


## Dev Environment Setup

```
export GOPATH=$HOME/<path_of_your_choice>

mkdir -p $HOME/<path_of_your_choice>/src

cd $HOME/<path_of_your_choice>/src

git clone git@github.com:harmony-one/harmony.git

cd harmony-benchmark

go get ./...
```
## Usage

### Running local test without db
```
./deploy.sh local_config.txt
```

### Running local test with db
```
./deploy.sh local_config.txt 1
```

## Testing

Make sure you the following command and make sure everything passed before submitting your code.

```
./test_before_submit.sh
```

## Linting

Make sure you the following command and make sure everything passes golint.

```
./lint_before_submit.sh
```

