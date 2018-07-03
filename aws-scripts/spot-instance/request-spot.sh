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
        \"Name\": \"RichardCodeDeployInstanceRole\" \
    }, \
    \"UserData\": \"`base64 -w 0 userdata.sh`\" \
  }" \
  --dry-run # uncomment this line to send a real request.
