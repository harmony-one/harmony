sudo yum update
sudo yum install wget
REGION=$(curl 16.9.254.169.254/latest/meta-data/placement/availability-zone | sed 's/[a-z]$//' )
cd /home/ec2-user
wget https://aws-codedeploy-$REGION.s3.amazonaws.com/lates/install
chmod +x ./install
sudo ./install auto