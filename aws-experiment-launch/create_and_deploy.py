import boto3
import argparse
import sys
import json
import time
import datetime
from threading import Thread
from Queue import Queue
import base64

REGION_NAME = 'region_name'
REGION_KEY = 'region_key'
REGION_SECURITY_GROUP = 'region_security_group'
REGION_HUMAN_NAME = 'region_human_name'
INSTANCE_TYPE = 't2.micro'
REGION_AMI = 'region_ami'
# USER_DATA = 'user-data.sh'
# UserData must be base64 encoded for spot instances.
with open("user-data.sh", "rb") as userdata_file:
    USER_DATA = base64.b64encode(userdata_file.read())

IAM_INSTANCE_PROFILE = 'BenchMarkCodeDeployInstanceProfile'
REPO = "simple-rules/harmony-benchmark"
APPLICATION_NAME = 'benchmark-experiments'
time_stamp = time.time()
CURRENT_SESSION = datetime.datetime.fromtimestamp(
    time_stamp).strftime('%H-%M-%S-%Y-%m-%d')
PLACEMENT_GROUP = "PLACEMENT-" + CURRENT_SESSION
NODE_NAME_SUFFIX = "NODE-" + CURRENT_SESSION

"""
TODO:

save NODE to disk, so that you can selectively only run deploy (not recreate instances). 
Right now all instances have "NODE" so this has uninted consequences of running on instances that were previous created.
Build (argparse,functions) support for 
1. run only create instance (multiple times)
2. run only codedeploy (multiple times)
3. run create instance followed by codedeploy

"""

### CREATE INSTANCES ###


def run_one_region_instances(config, region_number, number_of_instances, isOnDemand=True):
    #todo: explore the use ec2 resource and not client. e.g. create_instances --  Might make for better code.
    """
    e.g. ec2.create_instances
    """
    region_name = config[region_number][REGION_NAME]
    session = boto3.Session(region_name=region_name)
    ec2_client = session.client('ec2')
    if isOnDemand:
        NODE_NAME = create_instances(
            config, ec2_client, region_number, int(number_of_instances))
        print("Created %s in region %s"%(NODE_NAME,region_number))  ##REPLACE ALL print with logger
    else:
        response = request_spots(
            config, ec2_client, region_number, int(number_of_instances))
    return session


def create_instances(config, ec2_client, region_number, number_of_instances):
    NODE_NAME = region_number + "-" + NODE_NAME_SUFFIX
    response = ec2_client.run_instances(
        MinCount=number_of_instances,
        MaxCount=number_of_instances,
        ImageId=config[region_number][REGION_AMI],
        Placement={
            'AvailabilityZone': get_one_availability_zone(ec2_client),
        },
        SecurityGroups=[config[region_number][REGION_SECURITY_GROUP]],
        IamInstanceProfile={
            'Name': IAM_INSTANCE_PROFILE
        },
        KeyName=config[region_number][REGION_KEY],
        UserData=USER_DATA,
        InstanceType=INSTANCE_TYPE,
        TagSpecifications=[
            {
                'ResourceType': 'instance',
                'Tags': [
                    {
                        'Key': 'Name',
                        'Value': NODE_NAME
                    },
                ]
            },
        ],
        # We can also request spot instances this way but this way will block the
        # process until spot requests are fulfilled, otherwise it will throw exception
        # after 4 failed re-try.
        # InstanceMarketOptions= {
        #     'MarketType': 'spot',
        #     'SpotOptions': {
        #         'SpotInstanceType': 'one-time',
        #         'BlockDurationMinutes': 60,
        #     }
        # }
    )
    return NODE_NAME


def request_spots(config, ec2_client, region_number, number_of_instances):
    response = ec2_client.request_spot_instances(
        # DryRun=True,
        BlockDurationMinutes=60,
        InstanceCount=number_of_instances,
        LaunchSpecification={
            'SecurityGroups': [config[region_number][REGION_SECURITY_GROUP]],
            'IamInstanceProfile': {
                'Name': IAM_INSTANCE_PROFILE
            },
            'UserData': USER_DATA,
            'ImageId': config[region_number][REGION_AMI],
            'InstanceType': INSTANCE_TYPE,
            'KeyName': config[region_number][REGION_KEY],
            'Placement': {
                'AvailabilityZone': get_one_availability_zone(ec2_client)
            }
        }
    )
    return response


def get_availability_zones(ec2_client):
    response = ec2_client.describe_availability_zones()
    all_zones = []
    if response.get('AvailabilityZones', None):
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

#### CODEDEPLOY ###


def run_one_region_codedeploy(region_number, commitId):
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
    NODE_NAME = region_number + "-" + NODE_NAME_SUFFIX
    session = boto3.Session(region_name=region_name)
    ec2_client = session.client('ec2')
    filters = [{'Name': 'tag:Name','Values': [NODE_NAME]}]
    instance_ids = get_instance_ids(ec2_client.describe_instances(Filters=filters))
    
    print("Number of instances: %d" % len(instance_ids))

    print("Waiting for all %d instances in region %s to start running"%(len(instance_ids),region_number))
    waiter = ec2_client.get_waiter('instance_running')
    waiter.wait(InstanceIds=instance_ids)

    print("Waiting for all %d instances in region %s to be INSTANCE STATUS OK"%(len(instance_ids),region_number))
    waiter = ec2_client.get_waiter('instance_status_ok')
    waiter.wait(InstanceIds=instance_ids)

    print("Waiting for all %d instances in region %s to be SYSTEM STATUS OK"%(len(instance_ids),region_number))
    waiter = ec2_client.get_waiter('system_status_ok')
    waiter.wait(InstanceIds=instance_ids)

    codedeploy = session.client('codedeploy')
    application_name = APPLICATION_NAME
    deployment_group = APPLICATION_NAME + "-" + str(commitId) + "-1"
    repo = REPO

    print("Setting up to deploy commitId %s on region %s"%(commitId,region_number))
    response = get_application(codedeploy, application_name)
    response = get_deployment_group(
        codedeploy, region_number, application_name, deployment_group)
    depId = deploy(codedeploy, application_name,
                   deployment_group, repo, commitId)
    return region_number, depId


def get_deployment_group(codedeploy, region_number,application_name, deployment_group):
    NODE_NAME = region_number + "-" + NODE_NAME_SUFFIX
    response = codedeploy.list_deployment_groups(
        applicationName=application_name
    )
    if deployment_group in response['deploymentGroups']:
        return response
    else:
        response = codedeploy.create_deployment_group(
            applicationName=application_name,
            deploymentGroupName=deployment_group,
            deploymentConfigName='CodeDeployDefault.AllAtOnce',
            serviceRoleArn='arn:aws:iam::656503231766:role/BenchMarkCodeDeployServiceRole',
            deploymentStyle={
                'deploymentType': 'IN_PLACE',
                'deploymentOption': 'WITHOUT_TRAFFIC_CONTROL'
            },
            ec2TagFilters = [
                {
                    'Key': 'Name',
                    'Value': NODE_NAME,
                    'Type': 'KEY_AND_VALUE'
                }
            ]
        )
        return response


def get_application(codedeploy, application_name):
    response = codedeploy.list_applications()
    if application_name in response['applications']:
        return response
    else:
        response = codedeploy.create_application(
            applicationName=application_name,
            computePlatform='Server'
        )
    return response


def deploy(codedeploy, application_name, deployment_group, repo, commitId):
    """Deploy new code at specified revision to instance.

    arguments:
    - repo: GitHub repository path from which to get the code
    - commitId: commit ID to be deployed
    - wait: wait until the CodeDeploy finishes
    """
    print("Launching CodeDeploy with commit " + commitId)
    res = codedeploy.create_deployment(
        applicationName=application_name,
        deploymentGroupName=deployment_group,
        deploymentConfigName='CodeDeployDefault.AllAtOnce',
        description='benchmark experiments',
        revision={
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
    info = {'status': 'Created'}
    start = time.time()
    while info['status'] not in ('Succeeded', 'Failed', 'Stopped',) and (time.time() - start < 300.0):
        info = codedeploy.get_deployment(deploymentId=depId)['deploymentInfo']
        print(info['status'])
        time.sleep(15)
    if info['status'] == 'Succeeded':
        print("\nDeploy Succeeded")
        return depId
    else:
        print("\nDeploy Failed")
        print(info)
        return depId

def run_one_region_codedeploy_wrapper(region_number, commitId, queue):
    region_number, depId = run_one_region_codedeploy(region_number, commitId)
    queue.put((region_number, depId))

def launch_code_deploy(region_list, commitId):
    queue = Queue()
    jobs = []
    for i in range(len(region_list)):
        region_number = region_list[i]
        my_thread = Thread(target=run_one_region_codedeploy_wrapper, args=(
            region_number, commitId, queue))
        my_thread.start()
        jobs.append(my_thread)
    for my_thread in jobs:
        my_thread.join()
    results = [queue.get() for job in jobs]
    return results

##### UTILS ####


def get_instance_ids(describe_instances_response):
    instance_ids = []
    for reservation in describe_instances_response["Reservations"]:
        for instance in reservation["Instances"]:
            instance_ids.append(instance["InstanceId"])
    return instance_ids

def read_configuration_file(filename):
    config = {}
    with open(filename, 'r') as f:
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

def get_commitId(commitId):
    if commitId is None:
        commitId = run("git rev-list --max-count=1 HEAD",
                       hide=True).stdout.strip()
        print("Got newest commitId as " + commitId)
    return commitId

##### UTILS ####


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description='This script helps you start instances across multiple regions')
    parser.add_argument('--regions', type=str, dest='regions',
                        default='3', help="Supply a csv list of all regions")
    parser.add_argument('--instances', type=str, dest='numInstances',
                        default='1', help='number of instances')
    parser.add_argument('--configuration', type=str,
                        dest='config', default='configuration.txt')
    parser.add_argument('--commitId', type=str, dest='commitId',
                        default='1f7e6e7ca7cf1c1190cedec10e791c01a29971cf')
    args = parser.parse_args()
    config = read_configuration_file(args.config)
    region_list = args.regions.split(',')
    instances_list = args.numInstances.split(',')
    assert len(region_list) == len(instances_list), "number of regions: %d != number of instances per region: %d" % (
        len(region_list), len(intances_list))
    commitId = args.commitId
    for i in range(len(region_list)):
        region_number = region_list[i]
        number_of_instances = instances_list[i]
        session = run_one_region_instances(
            config, region_number, number_of_instances)
    results = launch_code_deploy(region_list, commitId)
    print(results)
