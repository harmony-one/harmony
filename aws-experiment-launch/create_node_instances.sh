# security_group_name=$1
# security_group_description=$2
# instanceName=$3
numOfInstances=$1
commitId=$2
region=$3

aws ec2 run-instances --image-id ami-a9d09ed1 --count $numOfInstances --instance-type t2.micro --key-name main --security-groups oregon-security-group \
--user-data user-data.sh --iam-instance-profile Name=CodeDeployDemo-EC2-Instance-Profile --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=Node}]' --compute-platform Server


#Create Application
aws deploy create-application --application-name myscriptdemo 

#Create Deployment Group
aws deploy create-deployment-group --application-name myscriptdemo --deployment-config-name CodeDeployDefault.AllAtOnce --ec2-tag-filters Key=Name,Value=Node,Type=KEY_AND_VALUE --deployment-group-name myfirstclideployment
--service-role-arn arn:aws:iam::656503231766:role/CodeDeployServiceRole


#Need Commit-ID.
aws deploy create-deployment \
  --application-name myscriptdemo \
  --deployment-config-name CodeDeployDefault.AllAtOnce \
  --deployment-group-name myfirstclideployment \
  --description "My GitHub deployment first cli demo" \
  --github-location repository=simple-rules/harmony-benchmark,commitId=$commitId