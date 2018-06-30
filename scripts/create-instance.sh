aws ec2 run-instances \
  --count 1 \
  --image-id ami-f2d3638a \
  --instance-type m3.medium \
  --key-name richard-spot-instance \
  --security-group-ids sg-06f90158506dca54f \
  --user-data file://instance-setup.sh