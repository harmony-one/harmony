# Swagger API Documentation

## Overview
The goal of the OpenAPI initiative is to provide comprehensive API documentation for Hamony API's including interative testing playground and the ability to generate clients or code from the specification moving forward.

### Phases 
The delivery will be broken down into the following milestones.
1. Generate [OpenAPI 3.0 specification](https://github.com/OAI/OpenAPI-Specification/blob/master/versions/3.0.0.md) for Harmony API's
2. Generate Postman Collection from Open API Specification
3. Generate an Interactive developer playground using Swagger
4. Generate the [OpenAPI 3.0 specification](https://github.com/OAI/OpenAPI-Specification/blob/master/versions/3.0.0.md) from the codebase
5. Generate clients automatically
6. Optionally generate code from the open API spec

### Prototyping
Initial prototyping is using a local deploy on an aws instance and can be viewd [here](http://54.201.207.240:8080/swaggerui/).


## Installation

* [install goswagger](https://goswagger.io/install.html)
For example on AWS linux
```
download_url=$(curl -s https://api.github.com/repos/go-swagger/go-swagger/releases/latest | \
  jq -r '.assets[] | select(.name | contains("'"$(uname | tr '[:upper:]' '[:lower:]')"'_amd64")) | .browser_download_url')
sudo curl -o /usr/local/bin/swagger -L'#' "$download_url"
sudo chmod +x /usr/local/bin/swagger
```

Installing Swagger UI
```
cd /home/ec2-user/go/src/github.com/harmony-one/harmony/swagge
git clone git@github.com:swagger-api/swagger-ui.git
mv ./swagger-ui/dist swaggerui 
rm -rf ./swagger-ui
cp swagger.json ./swaggerui/.

# build the [statik](https://github.com/rakyll/statik) file
go get github.com/rakyll/statik 
statik -src=/home/ec2-user/go/src/github.com/harmony-one/harmony/swagger/swaggerui
#statik -src=/Users/ribice/go/src/github.com/ribice/golang-swaggerui-example/cmd/swaggerui
```
* Initialize spec file
```
swagger init spec
```


## Running 

Building and serving swagger documentation
```
swagger generate spec -o swagger.json
swagger serve swagger.json --port 8082 --host=0.0.0.0 --no-open
```

## Contributing
* When documenting each API method use [operation_id](https://swagger.io/docs/specification/paths-and-operations/)


## Reference Material

* [Open API Specification](https://github.com/OAI/OpenAPI-Specification/blob/master/versions/3.0.0.md)
* [Swagger.io user guide](https://swagger.io/docs/specification/about/)
* [Swagger hub editor](https://app.swaggerhub.com/apis/wasdex/peerCount/0-oas3)
* [Swagger Supporting Fragment in Path Object](https://github.com/OAI/OpenAPI-Specification/issues/1635)
* [Swagger Support an operation having multiple specs per path](https://github.com/OAI/OpenAPI-Specification/issues/182)
* [Support for Open API spec 3.0 #1122](https://github.com/go-swagger/go-swagger/issues/1122)
* [gnostic - openapi to protobuf](https://github.com/googleapis/gnostic)
* [gnostic go generator](https://github.com/googleapis/gnostic-go-generator)

* [Generate Postman Collection from Open API](https://blog.postman.com/converting-openapi-specs-to-postman-collections/)

* [goswagger.io documentation](https://goswagger.io/)
* [goswagger.io github](https://github.com/go-swagger/go-swagger)
* [Create golang documentation with SwaggerUI](https://www.ribice.ba/swagger-golang/)
* [Generation from go source code](https://medium.com/@pedram.esmaeeli/generate-swagger-specification-from-go-source-code-648615f7b9d9)
* [Serve Swagger UI within go application](https://medium.com/@ribice/serve-swaggerui-within-your-golang-application-5486748a5ed4)
* Swagger URL - http://54.201.207.240:8080/swaggerui/
 
