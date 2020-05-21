# Swagger API Documentation

## Overview

The goal of the OpenAPI initiative is to
 
1. Provide comprehensive API documentation for Hamony API's
2. Provide an interactive UI for developers protoytping on Harmony
3. Provide an automation suite for testing
4. Provide  the ability to generate clients or code from the specification moving forward.

### Phases 
The delivery will be broken down into the following milestones.
1. Generate [OpenAPI 3.0 specification](https://github.com/OAI/OpenAPI-Specification/blob/master/versions/3.0.0.md) for Harmony API's
2. Generate Postman Collection from Open API Specification
3. Generate an Interactive developer playground using Swagger
4. Generate the [OpenAPI 3.0 specification](https://github.com/OAI/OpenAPI-Specification/blob/master/versions/3.0.0.md) from the codebase
5. Generate clients automatically
6. Optionally generate code from the open API spec

### Approach and Progress

Note: openapi typically has unique endpoints for each method as Harmony uses one endpoint for all API's workaround was done to create the documentation by suffixing method names.

1. Generate [OpenAPI 3.0 specification](https://github.com/OAI/OpenAPI-Specification/blob/master/versions/3.0.0.md) for Harmony API's

First approach - geneate from code
* This is the [Redux](https://prototype.johnwhitton.dev/docs).
* The site is generated using [runredux.sh](https://github.com/johnwhitton/harmony/blob/swagger-update/swagger/runredux.sh)
* `swagger generate spec -w ../cmd/harmony/ -o swagger.json` : reads through the codebase and genrates swagger.json
* Documentation was added to
  * [doc.go](https://github.com/johnwhitton/harmony/blob/swagger-update/docs/doc.go): high level project information  
  * [net.go](https://github.com/johnwhitton/harmony/blob/swagger-update/internal/hmyapi/apiv1/net.go#L10): API specific documentation
* Notes : goswagger only supports openAPI spec 2.0 and is not actively maintained

Second approach: document all APIs in swagger.yml and generate UI from this - see section 3

2. Generate Postman Collection from Open API Specification

* The generated Postman collection can be found [here](https://documenter.getpostman.com/view/6221615/Szt7BBFG)
* This was done by [manually importing the swagger.yml](https://stackoverflow.com/questions/39072216/how-to-import-swagger-apis-into-postman) and publishing it

3. Generate an Interactive developer playground using Swagger

* This is the [Swaagger](https://prototype.johnwhitton.dev/swagger/swaggerui/) site which is interactive
* The site is generated using [runSwagger.sh](https://github.com/johnwhitton/harmony/blob/swagger-update/swagger/runSwagger.sh)
* High level process flow is
  * manually document the APIs in swagger.yml
  * we clone the swager ui repository
  * point the ui to our swagger.yml by copyin a modified [index.html](https://github.com/johnwhitton/harmony/blob/swagger-update/swagger/index.html#L42)
  * turn the updated sit into a go package using statik
  * build the swagger ui and run it

4. Generate the [OpenAPI 3.0 specification](https://github.com/OAI/OpenAPI-Specification/blob/master/versions/3.0.0.md) from the codebase

This is still to be done and estimated two to three days to document all the API's

5. Generate clients automatically

Some prototyping has been done on this and is exciting for future SDK development. Please see the following

* [Support for Open API spec 3.0 #1122](https://github.com/go-swagger/go-swagger/issues/1122)
* [gnostic - openapi to protobuf](https://github.com/googleapis/gnostic)
* [gnostic go generator](https://github.com/googleapis/gnostic-go-generator)

And these medium articles

* [Generation from go source code](https://medium.com/@pedram.esmaeeli/generate-swagger-specification-from-go-source-code-648615f7b9d9)
* [Serve Swagger UI within go application](https://medium.com/@ribice/serve-swaggerui-within-your-golang-application-5486748a5ed4)

6. Optionally generate code from the open API spec

Once the API layer was completely specified other tools such as code generation could be used.

### Hosted Endpoints

Currently there are two hosted endpoints

1. [Swaagger](https://prototype.johnwhitton.dev/swagger/swaggerui/): This allows interactive prototyping for developers
2. [Redux](https://prototype.johnwhitton.dev/docs): This provides developer documentation but is not interactive.
3. [Postman - openapi](https://documenter.getpostman.com/view/6221615/Szt7BBFG)
4. [Postman complete documentation](https://documenter.getpostman.com/view/6221615/Szt7BB28)


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
cd /home/ec2-user/go/src/github.com/harmony-one/harmony/swagger
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
 
