import boto3
import argparse
import sys
import json
import time
import datetime
from threading import Thread
from Queue import Queue
import base64

from utils import utils


class InstanceResource:
    ON_DEMAND = 1
    SPOT_INSTANCE = 2
    SPOT_FLEET = 3

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

def run_one_region_instances(config, region_number, number_of_instances, instance_resource=InstanceResource.ON_DEMAND):
    region_name = config[region_number][utils.REGION_NAME]
    session = boto3.Session(region_name=region_name)
    ec2_client = session.client('ec2')
    print utils.get_one_availability_zone(ec2_client)
    if instance_resource == InstanceResource.ON_DEMAND:
        response, node_name_tag = create_instances(
            config, ec2_client, region_number, int(number_of_instances))
        print("Created %s in region %s" % (node_name_tag, region_number))
    else:
        return None, node_name_tag
    return response, node_name_tag


def create_instances(config, ec2_client, region_number, number_of_instances):
    node_name_tag = region_number + "-" + NODE_NAME_SUFFIX
    response = ec2_client.run_instances(
        MinCount=number_of_instances,
        MaxCount=number_of_instances,
        ImageId=config[region_number][utils.REGION_AMI],
        Placement={
            'AvailabilityZone': utils.get_one_availability_zone(ec2_client),
        },
        SecurityGroups=[config[region_number][utils.REGION_SECURITY_GROUP]],
        IamInstanceProfile={
            'Name': IAM_INSTANCE_PROFILE
        },
        KeyName=config[region_number][utils.REGION_KEY],
        UserData=USER_DATA,
        InstanceType=utils.INSTANCE_TYPE,
        TagSpecifications=[
            {
                'ResourceType': 'instance',
                'Tags': [
                    {
                        'Key': 'Name',
                        'Value': node_name_tag
                    },
                ]
            },
        ],
    )
    return response, node_name_tag

if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description='This script helps you start instances across multiple regions')
    parser.add_argument('--regions', type=str, dest='regions',
                        default='3', help="Supply a csv list of all regions")
    parser.add_argument('--instances', type=str, dest='numInstances',
                        default='1', help='number of instances')
    parser.add_argument('--configuration', type=str,
                        dest='config', default='configuration.txt')
    args = parser.parse_args()
    config = utils.read_region_config(args.config)
    region_list = args.regions.split(',')
    instances_list = args.numInstances.split(',')
    assert len(region_list) == len(instances_list), "number of regions: %d != number of instances per region: %d" % (len(region_list), len(instances_list))

    for i in range(len(region_list)):
        region_number = region_list[i]
        number_of_instances = instances_list[i]
        response, node_name_tag = run_one_region_instances(
            config, region_number, number_of_instances, InstanceResource.ON_DEMAND)
        if response:
            print utils.get_ip_list(response)

    # Enable the code below later.
    # results = launch_code_deploy(region_list, commitId)
    # print(results)
