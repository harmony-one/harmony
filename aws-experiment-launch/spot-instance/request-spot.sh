aws ec2 request-spot-instances \
  --instance-count 1 \
  --block-duration-minutes 60 \
  --launch-specification "{ \
    \"ImageId\": \"ami-f2d3638a\", \
    \"InstanceType\": \"m3.medium\", \
    \"SecurityGroups\": [ \
        \"richard-spot-instance SSH\" \
    ], \
    \"KeyName\": \"richard-spot-instance\", \
    \"IamInstanceProfile\": { \
        \"Name\": \"BenchMarkCodeDeployInstanceProfile\" \
    }, \
    \"UserData\": \"`base64 ../configs/userdata-commander.sh`\" \
  }" \
  #--dry-run # uncomment this line to send a real request.

# Note: on windows, you need to add `-w 0` to the base64 command"