# git clone git@github.com:swagger-api/swagger-ui.git
cp -R ./swagger-ui/dist swaggerui
# rm -rf ./swagger-ui
cp swagger.yml ./swaggerui/.
cp index.html ./swaggerui/.
statik -src=/home/ec2-user/go/src/github.com/harmony-one/harmony/swagger/swaggerui
rm -rf ./cmd/swaggerui/
mv statik ./cmd/swaggerui/
mv ./cmd/swaggerui/statik.go ./cmd/swaggerui/swaggerui.go
rm -rf swaggerui
go build -o tmp ./cmd/
./tmp

