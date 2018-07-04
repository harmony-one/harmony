aws ec2 request-spot-instances \
  --instance-count 1 \
  --block-duration-minutes 60 \
  --launch-specification file://launch_specs/latest/ohio.json
  # --dry-run # uncomment this line to send a real request.