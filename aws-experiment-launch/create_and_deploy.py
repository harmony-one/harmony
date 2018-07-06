import boto3
import argparse
import sys
import json
import time
import datetime
import logging
from threading import Thread
from Queue import Queue
import base64

LOGGER = logging.getLogger(__name__)

class InstanceResource:
    ON_DEMAND = 1
    SPOT_INSTANCE = 2
    SPOT_FLEET = 3


REGION_NAME = 'region_name'
REGION_KEY = 'region_key'
REGION_SECURITY_GROUP = 'region_security_group'
REGION_HUMAN_NAME = 'region_human_name'
INSTANCE_TYPE = 't2.small'
REGION_AMI = 'region_ami'
with open("user-data.sh", "r") as userdata_file:
    USER_DATA = userdata_file.read()

# UserData must be base64 encoded for spot instances.
USER_DATA_BASE64 = base64.b64encode(USER_DATA)

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


def create_one_region_instances(config, region_number, number_of_instances, instance_resource=InstanceResource.ON_DEMAND):
    #todo: explore the use ec2 resource and not client. e.g. create_instances --  Might make for better code.
    """
    e.g. ec2.create_instances
    """
    region_name = config[region_number][REGION_NAME]
    session = boto3.Session(region_name=region_name)
    ec2_client = session.client('ec2')
    if instance_resource == InstanceResource.ON_DEMAND:
        response, NODE_NAME = create_instances(
            config, ec2_client, region_number, int(number_of_instances))
        print("Created %s in region %s"%(NODE_NAME,region_number))  ##REPLACE ALL print with logger
    elif instance_resource == InstanceResource.SPOT_INSTANCE:
        response, NODE_NAME = request_spot_instances(
            config, ec2_client, region_number, int(number_of_instances))
    else:
        response, NODE_NAME = request_spot_fleet(
            config, ec2_client, region_number, int(number_of_instances))
    return response, NODE_NAME


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
    return response, NODE_NAME


def request_spot_instances(config, ec2_client, region_number, number_of_instances):
    NODE_NAME = region_number + "-" + NODE_NAME_SUFFIX
    response = ec2_client.request_spot_instances(
        # DryRun=True,
        BlockDurationMinutes=60,
        InstanceCount=number_of_instances,
        LaunchSpecification={
            'SecurityGroups': [config[region_number][REGION_SECURITY_GROUP]],
            'IamInstanceProfile': {
                'Name': IAM_INSTANCE_PROFILE
            },
            'UserData': USER_DATA_BASE64,
            'ImageId': config[region_number][REGION_AMI],
            'InstanceType': INSTANCE_TYPE,
            'KeyName': config[region_number][REGION_KEY],
            'Placement': {
                'AvailabilityZone': get_one_availability_zone(ec2_client)
            }
        }
    )
    return response, NODE_NAME


def request_spot_fleet(config, ec2_client, region_number, number_of_instances):
    NODE_NAME = region_number + "-" + NODE_NAME_SUFFIX
    # https://boto3.readthedocs.io/en/latest/reference/services/ec2.html#EC2.Client.request_spot_fleet
    response = ec2_client.request_spot_fleet(
        # DryRun=True,
        SpotFleetRequestConfig={
            # https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-fleet.html#spot-fleet-allocation-strategy
            'AllocationStrategy': 'diversified',
            # 'IamFleetRole': IAM_INSTANCE_PROFILE, // TODO@ricl, create fleet role.
            'LaunchSpecifications': [
                {
                    'SecurityGroups': [
                        {
                            'GroupName': config[region_number][REGION_SECURITY_GROUP]
                        }
                    ],
                    'IamInstanceProfile': {
                        'Name': IAM_INSTANCE_PROFILE
                    },
                    'ImageId': config[region_number][REGION_AMI],
                    'InstanceType': INSTANCE_TYPE,
                    'KeyName': config[region_number][REGION_KEY],
                    'Placement': {
                        'AvailabilityZone': get_one_availability_zone(ec2_client)
                    },
                    'UserData': USER_DATA,
                    # 'WeightedCapacity': 123.0,
                    'TagSpecifications': [
                        {
                            'ResourceType': 'instance',
                            'Tags': [
                                {
                                    'Key': 'Name',
                                    'Value': NODE_NAME
                                },
                            ]
                        }
                    ]
                },
            ],
            # 'SpotPrice': 'string', # The maximum price per unit hour that you are willing to pay for a Spot Instance. The default is the On-Demand price.
            'TargetCapacity': 1,
            'OnDemandTargetCapacity': 0,
            'Type': 'maintain',
        }
    )
    return response, NODE_NAME


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
    deployment_group = APPLICATION_NAME + "-" + str(commitId)[6] + "-" + CURRENT_SESSION
    repo = REPO

    print("Setting up to deploy commitId %s on region %s"%(commitId,region_number))
    response = get_application(codedeploy, application_name)
    deployment_group = get_deployment_group(
        codedeploy, region_number, application_name, deployment_group)
    depId = deploy(codedeploy, application_name,
                   deployment_group, repo, commitId)
    return region_number, depId


def get_deployment_group(codedeploy, region_number,application_name, deployment_group):
    NODE_NAME = region_number + "-" + NODE_NAME_SUFFIX
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
    return deployment_group


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
    while info['status'] not in ('Succeeded', 'Failed', 'Stopped',) and (time.time() - start < 600.0):
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

def get_public_ips(response):
    ips = []
    for instance in response['Instances']:
        ips.append(instance['PublicIpAddress'])
    return ips

def generate_config_file(shard_num, client_num, ips, config_filename):
    if len(ips) < shard_num * 2 + client_num:
        print("Not enough nodes to generate a config file")
        return False

    # Create ip for clients.
    client_id, leader_id, validator_id = 0, 0, 0
    with open(config_filename, "w") as fout:
        for i in range(len(ips)):
            if client_id < client_num:
                fout.write("%s 9000 client %d\n" % (ips[i], client_id % shard_num))
                client_id = client_id + 1
            elif leader_id < shard_num:
                fout.write("%s 9000 leader %d\n" % (ips[i], leader_id))
                leader_id = leader_id + 1
            else:
                fout.write("%s 9000 validator %d\n" % (ips[i], validator_id % shard_num))
                validator_id = validator_id + 1

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
    parser.add_argument('--shard_num', type=int, dest='shard_number', default=1, help='number of shards')
    parser.add_argument('--client_num', type=int, dest='client_number', default=1, help='number of clients')
    parser.add_argument('--config_filename', type=str, dest='config_filename', default="benchmark_config.txt",
                        help='The filename of the config')

    args = parser.parse_args()
    config = read_configuration_file(args.config)
    region_list = args.regions.split(',')
    instances_list = args.numInstances.split(',')
    assert len(region_list) == len(instances_list), "number of regions: %d != number of instances per region: %d" % (
        len(region_list), len(instances_list))
    commitId = args.commitId
    for i in range(len(region_list)):
        region_number = region_list[i]
        number_of_instances = instances_list[i]
        response, node_name_tag = create_one_region_instances(
            config, region_number, number_of_instances, InstanceResource.ON_DEMAND)

        ips = get_public_ips(response)
        generate_config_file(args.shard_num, args.client_num, ips, args.config_filename)
        
    # Enable the below code when creating instances working.
    # results = launch_code_deploy(region_list, commitId)
    # print(results)
