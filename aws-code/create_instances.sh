# security_group_name=$1
# security_group_description=$2
aws ec2 create-security-group --group-name mcDG --description "mcdg"
aws ec2 authorize-security-group-ingress --group-name MySecurityGroup --protocol tcp  --port all --cidr 0.0.0.0/0

aws ec2 run-instances --image-id ami-a9d09ed1 --count 1 --instance-type t2.micro --key-name main --security-group-ids mcDG \
--user-data user-data.sh --iam-instance-profile Name=CodeDeployDemo-EC2-Instance-Profile