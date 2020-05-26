# git clone git@github.com:swagger-api/swagger-ui.git
rm -rf swaggerui
mkdir swaggerui
cp -rf ./swagger-ui/dist swaggerui
# rm -rf ./swagger-ui
cp swagger.yml ./swaggerui/.
cp index.html ./swaggerui/.
statik -src=/home/ec2-user/go/src/github.com/harmony-one/harmony/docs/swaggerui -p docs
mv ./statik/statik.go swaggerui.go
rm -rf statik
rm -rf swaggerui
