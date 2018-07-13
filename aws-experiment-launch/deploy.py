import argparse
import base64
import boto3
import datetime
import json
import logging
import os
import subprocess
import sys
import threading
import time

from utils import utils

logging.basicConfig(level=logging.INFO, format='%(threadName)s %(asctime)s - %(name)s - %(levelname)s - %(message)s')
LOGGER = logging.getLogger(__file__)
LOGGER.setLevel(logging.INFO)

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

MAX_INSTANCES_FOR_DEPLOYMENT = 500

def run_one_region_codedeploy(region_number, region_config, node_name_tag, commit_id):
    ec2_client, session = utils.create_ec2_client(region_number, region_config)
    filters = [{'Name': 'tag:Name','Values': [node_name_tag]}]
    instance_ids = utils.get_instance_ids(ec2_client.describe_instances(Filters=filters))

    total_instances = len(instance_ids)
    thread_pool = []
    i = 0
    while i < total_instances:
        j = min(total_instances, i + MAX_INSTANCES_FOR_DEPLOYMENT)
        t = threading.Thread(target=run_one_region_codedeploy_with_max_500_instances, args=(
                            ec2_client, session, region_number, instance_ids[i:j],
                            node_name_tag, commit_id, i))
        t.start()
        thread_pool.append(t)
        i = i + MAX_INSTANCES_FOR_DEPLOYMENT
    for t in thread_pool:
        t.join()


def run_one_region_codedeploy_with_max_500_instances(ec2_client, session, region_number,
        instance_ids, node_name_tag, commit_id, tag_id):
    LOGGER.info("Number of instances: %d" % len(instance_ids))

    LOGGER.info("Waiting for %d instances in region %s to be in RUNNING" % (len(instance_ids), region_number))
    utils.run_waiter_for_status(ec2_client, 'instance_running', instance_ids)
    # waiter = ec2_client.get_waiter('instance_running')
    # waiter.wait(InstanceIds=instance_ids)

    LOGGER.info("Waiting for %d instances in region %s with status OK"% (len(instance_ids), region_number))
    utils.run_waiter_for_status(ec2_client, 'instance_status_ok', instance_ids)
    # waiter = ec2_client.get_waiter('instance_status_ok')
    # waiter.wait(InstanceIds=instance_ids)

    LOGGER.info("Waiting for %d instances in region %s with system in OK"% (len(instance_ids), region_number))
    utils.run_waiter_for_status(ec2_client, 'system_status_ok', instance_ids)
    # waiter = ec2_client.get_waiter('system_status_ok')
    # waiter.wait(InstanceIds=instance_ids)

    codedeploy = session.client('codedeploy')
    application_name = APPLICATION_NAME
    deployment_group_name = APPLICATION_NAME + "-" + commit_id[:6] + "-" + str(tag_id) + "-" + CURRENT_SESSION
    repo = REPO

    LOGGER.info("Setting up to deploy commit_id %s on region %s" % (commit_id, region_number))
    utils.get_application(codedeploy, application_name)
    deployment_group_id = utils.create_deployment_group(
        codedeploy, region_number, application_name, deployment_group_name, node_name_tag)
    if deployment_group_id:
        LOGGER.info("Created deployment group with id %s" % deployment_group_id)
    else:
        LOGGER.info("Created deployment group with name %s was created" % deployment_group_name)
    deployment_id, status = deploy(codedeploy, application_name, deployment_group_name, repo, commit_id)
    return region_number, deployment_id, status


def deploy(codedeploy, application_name, deployment_group, repo, commit_id):
    """Deploy new code at specified revision to instance.

    arguments:
    - repo: GitHub repository path from which to get the code
    - commit_id: commit ID to be deployed
    - wait: wait until the CodeDeploy finishes
    """
    LOGGER.info("Launching CodeDeploy with commit " + commit_id)
    response = codedeploy.create_deployment(
        applicationName=application_name,
        deploymentGroupName=deployment_group,
        deploymentConfigName='CodeDeployDefault.AllAtOnce',
        description='benchmark experiments',
        revision={
            'revisionType': 'GitHub',
            'gitHubLocation': {
                    'repository': repo,
                    'commitId': commit_id,
            }
        }
    )
    if response:
        LOGGER.info("Deployment returned with deployment id: " + response["deploymentId"])
        deployment_id = response["deploymentId"]
    else:
        LOGGER.error("Deployment failed.")
        return None, None
    time.sleep(1)
    start_time = time.time()
    status = None
    while time.time() - start_time < 600:
        response = codedeploy.get_deployment(deploymentId=deployment_id)
        if response and response.get('deploymentInfo'):
            status = response['deploymentInfo']['status']
            if status in ('Succeeded', 'Failed', 'Stopped'):
                break
        time.sleep(2)
    if status:
        LOGGER.info("Deployment group %s finished with status %s" % (deployment_group, status))
    else:
        LOGGER.info("Deployment status: time out")
    return deployment_id, status

def launch_code_deploy(region_list, region_config, commit_id):
    thread_pool = []
    for region_tuppple in region_list:
        # node_name_tag comes first.
        node_name_tag, region_number = region_tuppple
        t = threading.Thread(target=run_one_region_codedeploy, args=(
            region_number, region_config, node_name_tag, commit_id))
        t.start()
        thread_pool.append(t)
    for t in thread_pool:
        t.join()
    LOGGER.info("Finished.")

def get_head_commit_id():
    git_head_hash = None
    try:
        process = subprocess.Popen(['git', 'rev-parse', 'HEAD'], shell=False, stdout=subprocess.PIPE)
        git_head_hash = process.communicate()[0].strip()
    finally:
        return git_head_hash 


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description='This script helps you start instances across multiple regions')
    parser.add_argument('--instance_output', type=str, dest='instance_output',
                        default='instance_output.txt',
                        help='the file contains node_name_tag and region number of created instances.')
    parser.add_argument('--region_config', type=str, dest='region_config', default='configuration.txt')
    parser.add_argument('--commit_id', type=str, dest='commit_id',
                        default='f092d25d7a814622079fe92e9b36e10e46bc0d97')
    args = parser.parse_args()
    LOGGER.info("********* MAKE SURE YOU'RE RUNNING under harmony-benchmark code base *********")
    commit_id = get_head_commit_id() or args.commit_id

    if not os.path.isfile(args.instance_output) or not commit_id:
        LOGGER.info("%s does not exist" % args.instance_output)
        sys.exit(1)

    with open(args.instance_output, "r") as fin:
        region_list = [line.split(" ") for line in fin.readlines()]
        region_list = [(item[0].strip(), item[1].strip()) for item in region_list]
        launch_code_deploy(region_list, args.region_config, commit_id)
