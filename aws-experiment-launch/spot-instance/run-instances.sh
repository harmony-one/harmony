aws ec2 run-instances \
  --count 1 \
  --image-id "ami-f2d3638a" \
  --instance-type "t2.micro" \
  --key-name "richard-spot-instance" \
  --security-groups "richard-spot-instance SSH" \
  --iam-instance-profile "{ \
    \"Name\": \"BenchMarkCodeDeployInstanceProfile\" \
  }" \
  --user-data "`base64 ../configs/userdata-commander.sh`"