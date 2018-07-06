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

def run_one_region_codedeploy(region_number, commit_id):
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
    deployment_group = APPLICATION_NAME + "-" + str(commit_id)[6] + "-" + CURRENT_SESSION
    repo = REPO

    print("Setting up to deploy commit_id %s on region %s"%(commit_id,region_number))
    response = get_application(codedeploy, application_name)
    deployment_group = get_deployment_group(
        codedeploy, region_number, application_name, deployment_group)
    depId = deploy(codedeploy, application_name,
                   deployment_group, repo, commit_id)
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


def deploy(codedeploy, application_name, deployment_group, repo, commit_id):
    """Deploy new code at specified revision to instance.

    arguments:
    - repo: GitHub repository path from which to get the code
    - commit_id: commit ID to be deployed
    - wait: wait until the CodeDeploy finishes
    """
    print("Launching CodeDeploy with commit " + commit_id)
    res = codedeploy.create_deployment(
        applicationName=application_name,
        deploymentGroupName=deployment_group,
        deploymentConfigName='CodeDeployDefault.AllAtOnce',
        description='benchmark experiments',
        revision={
            'revisionType': 'GitHub',
            'gitHubLocation': {
                    'repository': repo,
                    'commit_id': commit_id,
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

def run_one_region_codedeploy_wrapper(region_number, commit_id, queue):
    region_number, depId = run_one_region_codedeploy(region_number, commit_id)
    queue.put((region_number, depId))

def launch_code_deploy(region_list, commit_id):
    queue = Queue()
    jobs = []
    for i in range(len(region_list)):
        region_number = region_list[i]
        my_thread = Thread(target=run_one_region_codedeploy_wrapper, args=(
            region_number, commit_id, queue))
        my_thread.start()
        jobs.append(my_thread)
    for my_thread in jobs:
        my_thread.join()
    results = [queue.get() for job in jobs]
    return results

def get_instance_ids(describe_instances_response):
    instance_ids = []
    for reservation in describe_instances_response["Reservations"]:
        for instance in reservation["Instances"]:
            instance_ids.append(instance["InstanceId"])
    return instance_ids

if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description='This script helps you start instances across multiple regions')
    parser.add_argument('--regions', type=str, dest='regions',
                        default='3', help="Supply a csv list of all regions")
    parser.add_argument('--instances', type=str, dest='numInstances',
                        default='1', help='number of instances')
    parser.add_argument('--configuration', type=str,
                        dest='config', default='configuration.txt')
    parser.add_argument('--commit_id', type=str, dest='commit_id',
                        default='1f7e6e7ca7cf1c1190cedec10e791c01a29971cf')
    args = parser.parse_args()
    config = utils.read_region_config(args.config)
    region_list = [item.strip() for item in args.regions.split(',')]
    instances_list = [item.strip() for item in args.numInstances.split(',')]
    assert len(region_list) == len(instances_list), "number of regions: %d != number of instances per region: %d" % (
        len(region_list), len(instances_list))
    commit_id = args.commit_id

    results = launch_code_deploy(region_list, commit_id)
    print(results)
