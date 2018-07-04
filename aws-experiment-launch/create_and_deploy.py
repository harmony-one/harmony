import boto3
import argparse
import sys
import json
import time
import datetime

REGION_NAME = 'region_name'
REGION_KEY = 'region_key'
REGION_SECURITY_GROUP = 'region_security_group'
REGION_HUMAN_NAME = 'region_human_name'
INSTANCE_TYPE = 't2.micro'
REGION_AMI = 'region_ami'
USER_DATA = 'user-data.sh'
IAM_INSTANCE_PROFILE = 'BenchMarkCodeDeployInstanceProfile'
REPO = "simple-rules/harmony-benchmark"
APPLICATION_NAME = 'benchmark-experiments'
time_stamp = time.time()
CURRENT_SESSION = datetime.datetime.fromtimestamp(time_stamp).strftime('%H-%M-%S-%Y-%m-%d')
PLACEMENT_GROUP = "PLACEMENT-" + CURRENT_SESSION
NODE_VALUE = "NODE-" + CURRENT_SESSION

"""
TODO:
NODE = region_number + NODE_VALUE
Use that to retrieve ids, only deploy on specific nodes (right now it deploys everywere), remove placement group. 
save NODE to disk, so that you can selectively only run deploy (not recreate instances). 
Right now all instances have "NODE" so this has uninted consequences of running on instances that were previous created.
Build (argparse,functions) support for 
1. run only create instance (multiple times)
2. run only codedeploy (multiple times)
3. run create instance followed by codedeploy

"""
def get_instance_ids(response):
    instance_ids = []
    for reservation in response["Reservations"]:
        for instance in reservation["Instances"]:
            instance_ids.append(instance["InstanceId"])
    return instance_ids   

def run_one_region_instances(config,region_number,number_of_instances):
    #todo: explore the use ec2 resource and not client. e.g. create_instances --  Might make for better code.
    """
    e.g. ec2.create_instances
    """
    region_name = config[region_number][REGION_NAME]
    session = boto3.Session(region_name=region_name)
    ec2_client = session.client('ec2')
    response,placement = create_instances(config,ec2_client,region_number,int(number_of_instances))
    print(placement)
    return session,placement

def run_one_region_codedeploy(region_number,placement_group,commitId):
    #todo: explore the use ec2 resource and not client. e.g. create_instances --  Might make for better code.
    """
    for getting instance ids:---
    ec2 = boto3.resource('ec2', region_name=region_name])
    result = ec2.instances.filter(Filters=[{'Name': 'instance-state-name', 'Values': ['running']}])
    for instance in result:
        instances.append(instance.id)
    
    for getting public ips : --
        ec2 = boto3.resource('ec2')
        instance
    """
    region_name = config[region_number][REGION_NAME]
    session = boto3.Session(region_name=region_name)
    ec2_client = session.client('ec2')
    response = ec2_client.describe_instances(
        Filters = [
            {
                'Name': 'placement-group-name',
                'Values' : [
                        placement_group
                ]
            }
        ]
    )
    instance_ids = get_instance_ids(response)
    
    print("Waiting for all instances to start running")
    waiter = ec2_client.get_waiter('instance_running')
    waiter.wait(InstanceIds=instance_ids)

    print("Waiting for all instances to be status ok")
    waiter = ec2_client.get_waiter('instance_status_ok')
    waiter.wait(InstanceIds=instance_ids)

    print("Waiting for system to be status ok")
    waiter = ec2_client.get_waiter('system_status_ok')
    waiter.wait(InstanceIds=instance_ids)

    codedeploy = session.client('codedeploy')
    application_name = APPLICATION_NAME
    deployment_group = APPLICATION_NAME + "-" + str(commitId)
    repo = REPO
    response = get_application(codedeploy,application_name)
    response  = get_deployment_group(codedeploy,application_name,deployment_group)
    deploy(codedeploy, application_name, deployment_group, repo, commitId, wait=True)
    return response
    
def get_availability_zones(ec2_client):
    response = ec2_client.describe_availability_zones()
    all_zones = []
    if response.get('AvailabilityZones',None):
       region_info = response.get('AvailabilityZones')
       for info in region_info:
           if info['State'] == 'available':
               all_zones.append(info['ZoneName'])
    return all_zones

def get_one_availability_zone(ec2_client):
    all_zones = get_availability_zones(ec2_client)
    if len(all_zones) > 0:
        return all_zones[0]
    else:
        print("No availability zone for this region")
        sys.exit()

def create_instances(config,ec2_client,region_number,number_of_instances):
    placement_group = region_number + "-" + PLACEMENT_GROUP
    response = ec2_client.create_placement_group(
    GroupName = placement_group,
    Strategy = 'spread'
    ) 
    response = ec2_client.run_instances(
        MinCount = number_of_instances,
        MaxCount = number_of_instances,
        ImageId = config[region_number][REGION_AMI],
        Placement = {
        'AvailabilityZone': get_one_availability_zone(ec2_client),
        'GroupName': placement_group
        },
        SecurityGroups = [config[region_number][REGION_SECURITY_GROUP]],
        IamInstanceProfile = {
            'Name' : IAM_INSTANCE_PROFILE
        },
        KeyName = config[region_number][REGION_KEY],
        UserData = USER_DATA,
        InstanceType = INSTANCE_TYPE,
        TagSpecifications = [
          {
            'ResourceType' : 'instance',
            'Tags': [
                {
                    'Key': 'Name',
                    'Value': 'Node'
                },
            ]
          },
        ]
    )
    return response,placement_group

def get_deployment_group(codedeploy,application_name,deployment_group):
    response = codedeploy.list_deployment_groups(
        applicationName = application_name
    )
    if deployment_group in response['deploymentGroups']:
        return response
    else: 
        response = codedeploy.create_deployment_group(
                applicationName = application_name,
                deploymentGroupName = deployment_group,
                deploymentConfigName = 'CodeDeployDefault.AllAtOnce',
                serviceRoleArn = 'arn:aws:iam::656503231766:role/BenchMarkCodeDeployServiceRole',
                deploymentStyle={
                    'deploymentType': 'IN_PLACE',
                    'deploymentOption': 'WITHOUT_TRAFFIC_CONTROL'
                },
                ec2TagSet={
                    'ec2TagSetList': [
                        [
                        {
                            'Key': 'Name',
                            'Value': 'Node',
                            'Type': 'KEY_AND_VALUE'
                        },
                        ],
                    ]
                }
        )
        return response

def get_commitId(commitId):
    if commitId is None:
        commitId = run("git rev-list --max-count=1 HEAD",
                       hide=True).stdout.strip()
        print("Got newest commitId as " + commitId)
    return commitId

def get_application(codedeploy,application_name):
    response = codedeploy.list_applications()
    if application_name in response['applications']:
        return response
    else:
        response = codedeploy.create_application(
        applicationName= application_name,
        computePlatform='Server'
    )
    return response

def deploy(codedeploy, application_name,deployment_group,repo, commitId, wait=True):
    """Deploy new code at specified revision to instance.

    arguments:
    - repo: GitHub repository path from which to get the code
    - commitId: commit ID to be deployed
    - wait: wait until the CodeDeploy finishes
    """
    print("Launching CodeDeploy with commit " + commitId)
    res = codedeploy.create_deployment(
            applicationName = application_name,
            deploymentGroupName = deployment_group,
            deploymentConfigName = 'CodeDeployDefault.AllAtOnce',
            description = 'benchmark experiments',
            revision = {
                'revisionType': 'GitHub',
                'gitHubLocation': {
                'repository': repo,
                'commitId': commitId,
                }
            }
        )
    depId = res["deploymentId"]
    print("Deployment ID: " + depId)

    # The deployment is launched at this point, so exit unless asked to wait
    # until it finishes
    if not wait:
        return
    info = {'status': 'Created'}
    start = time.time()
    while info['status'] not in ('Succeeded', 'Failed', 'Stopped',) and (time.time() - start < 300.0):
        info = codedeploy.get_deployment(deploymentId=depId)['deploymentInfo']
        print(info['status'])
        time.sleep(15)
    if info['status'] == 'Succeeded':
        print("\nDeploy Succeeded")
    else:
        print("\nDeploy Failed")
        print(info)

def read_configuration_file(filename):
    config = {}
    with open(filename,'r') as f:
        for myline in f:
            mylist = myline.strip().split(',')
            region_num = mylist[0]
            config[region_num] = {}
            config[region_num][REGION_NAME] = mylist[1]
            config[region_num][REGION_KEY] = mylist[2]
            config[region_num][REGION_SECURITY_GROUP] = mylist[3]
            config[region_num][REGION_HUMAN_NAME] = mylist[4]
            config[region_num][REGION_AMI] = mylist[5]
    return config

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='This script helps you start instances across multiple regions')
    parser.add_argument('--regions',type=str,dest='regions',default='3',help="Supply a csv list of all regions")
    parser.add_argument('--instances', type=str,dest='numInstances',default='1',help='number of instances')
    parser.add_argument('--configuration',type=str,dest='config',default='configuration.txt')
    parser.add_argument('--commitId',type=str,dest='commitId',default='1f7e6e7ca7cf1c1190cedec10e791c01a29971cf')
    args = parser.parse_args()
    config = read_configuration_file(args.config)
    region_list = args.regions.split(',')
    instances_list = args.numInstances.split(',')
    assert len(region_list) == len(instances_list),"number of regions: %d != number of instances per region: %d" % (len(region_list),len(intances_list))
    commitId = args.commitId
    placement_groups = []

    for i in range(len(region_list)):
        region_number = region_list[i]
        number_of_instances = instances_list[i]
        session,placement_group = run_one_region_instances(config,region_number,number_of_instances)
        placement_groups.append(placement_group)
        
    for i in range(len(region_list)):
        region_number = region_list[i]
        placement_group = placement_groups[i]
        run_one_region_codedeploy(region_number,placement_group,commitId)

   