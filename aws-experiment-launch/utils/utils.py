import boto3
import datetime
import json
import sys
import time
import base64
import threading


MAX_INTANCES_FOR_WAITER = 100
MAX_INSTANCES_FOR_DEPLOYMENT = 500
REGION_NAME = 'region_name'
REGION_KEY = 'region_key'
REGION_SECURITY_GROUP = 'region_security_group'
REGION_SECURITY_GROUP_ID = 'region_security_group_id'
REGION_HUMAN_NAME = 'region_human_name'
INSTANCE_TYPE = 't2.micro'
REGION_AMI = 'region_ami'
IAM_INSTANCE_PROFILE = 'BenchMarkCodeDeployInstanceProfile'
time_stamp = time.time()
CURRENT_SESSION = datetime.datetime.fromtimestamp(
    time_stamp).strftime('%H-%M-%S-%Y-%m-%d')
NODE_NAME_SUFFIX = "NODE-" + CURRENT_SESSION

def get_node_name_tag(region_number):
    return region_number + "-" + NODE_NAME_SUFFIX

def get_node_name_tag2(region_number, tag):
    return region_number + "-" + NODE_NAME_SUFFIX + "-" + str(tag)

with open("userdata-soldier.sh", "r") as userdata_file:
    USER_DATA = userdata_file.read()

# UserData must be base64 encoded for spot instances.
USER_DATA_BASE64 = base64.b64encode(USER_DATA)

def create_client(config, region_number):
    region_name = config[region_number][REGION_NAME]
    # Create session.
    session = boto3.Session(region_name=region_name)
    # Create a client.
    return session.client('ec2')

def read_region_config(region_config='configuration.txt'):
    global CONFIG
    config = {}
    with open(region_config, 'r') as f:
        for myline in f:
            mylist = [item.strip() for item in myline.strip().split(',')]
            region_num = mylist[0]
            config[region_num] = {}
            config[region_num][REGION_NAME] = mylist[1]
            config[region_num][REGION_KEY] = mylist[2]
            config[region_num][REGION_SECURITY_GROUP] = mylist[3]
            config[region_num][REGION_HUMAN_NAME] = mylist[4]
            config[region_num][REGION_AMI] = mylist[5]
            config[region_num][REGION_SECURITY_GROUP_ID] = mylist[6]
    CONFIG = config
    return config

def get_ip_list(response):
    if response.get('Instances', None):
        return [instance.get('PublicIpAddress', None) for instance in response['Instances']]
    else:
        return []

def create_ec2_client(region_number, region_config):
    config = read_region_config(region_config)
    region_name = config[region_number][REGION_NAME]
    session = boto3.Session(region_name=region_name)
    return session.client('ec2'), session

def collect_public_ips_from_ec2_client(ec2_client, node_name_tag):
    filters = [{'Name': 'tag:Name','Values': [node_name_tag]}]
    response = ec2_client.describe_instances(Filters=filters)
    ip_list = []
    if response.get('Reservations'):
        for reservation in response[u'Reservations']:
            ip_list.extend(instance['PublicIpAddress'] for instance in reservation['Instances'] if instance.get('PublicIpAddress'))
    return ip_list

def collect_public_ips(region_number, node_name_tag, region_config):
    ec2_client, _ = create_ec2_client(region_number, region_config)
    ip_list = collect_public_ips_from_ec2_client(ec2_client, node_name_tag)
    return ip_list

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

def create_deployment_group(codedeploy, region_number,application_name, deployment_group_name, node_name_tag):
    response = codedeploy.list_deployment_groups(applicationName=application_name)
    if response.get('deploymentGroups') and (deployment_group_name in response['deploymentGroups']):
        return None
    else:
        response = codedeploy.create_deployment_group(
            applicationName=application_name,
            deploymentGroupName=deployment_group_name,
            deploymentConfigName='CodeDeployDefault.AllAtOnce',
            serviceRoleArn='arn:aws:iam::656503231766:role/BenchMarkCodeDeployServiceRole',
            deploymentStyle={
                'deploymentType': 'IN_PLACE',
                'deploymentOption': 'WITHOUT_TRAFFIC_CONTROL'
            },
            ec2TagFilters = [
                {
                    'Key': 'Name',
                    'Value': node_name_tag,
                    'Type': 'KEY_AND_VALUE'
                }
            ]
        )
        return response['deploymentGroupId']

def generate_distribution_config2(region_number, node_name_tag, region_config,
                                  shard_number, client_number, distribution_config):
    ip_list = collect_public_ips(region_number, node_name_tag, region_config)
    generate_distribution_config(shard_number, client_number, ip_list, distribution_config)

def generate_distribution_config3(shard_number, client_number, ip_list_file, distribution_config):
    with open(ip_list_file, "r") as fin:
        lines = fin.readlines()
        ip_list = [line.strip() for line in lines]
    generate_distribution_config(shard_number, client_number, ip_list, distribution_config)

def generate_distribution_config(shard_number, client_number, ip_list, distribution_config):
    if len(ip_list) < shard_number * 2 + client_number + 1:
        print("Not enough nodes to generate a config file")
        return False

    # Create ip for clients.
    commander_id, client_id, leader_id, validator_id = 0, 0, 0, 0
    validator_number = len(ip_list) - client_number - shard_number - 1
    with open(distribution_config, "w") as fout:
        for i in range(len(ip_list)):
            ip, node_name_tag = ip_list[i].split(" ")
            if commander_id < 1:
                fout.write("%s 9000 commander %d %s\n" % (ip, commander_id, node_name_tag))
                commander_id = commander_id + 1
            elif leader_id < shard_number:
                fout.write("%s 9000 leader %d %s\n" % (ip, leader_id, node_name_tag))
                leader_id = leader_id + 1
            elif validator_id < validator_number:
                fout.write("%s 9000 validator %d %s\n" % (ip, validator_id % shard_number, node_name_tag))
                validator_id = validator_id + 1
            else:
                fout.write("%s 9000 client %d %s\n" % (ip, client_id % shard_number, node_name_tag))
                client_id = client_id + 1

def get_availability_zones(ec2_client):
    response = ec2_client.describe_availability_zones()
    all_zones = []
    if response.get('AvailabilityZones', None):
        all_zones = [info['ZoneName'] for info in response.get('AvailabilityZones') if info['State'] == 'available']
    return all_zones

def get_one_availability_zone(ec2_client):
    time.sleep(1)
    all_zones = get_availability_zones(ec2_client)
    if len(all_zones) > 0:
        return all_zones[0]
    else:
        return None

def get_instance_ids2(ec2_client, node_name_tag):
    time.sleep(5)
    filters = [{'Name': 'tag:Name','Values': [node_name_tag]}]
    return get_instance_ids(ec2_client.describe_instances(Filters=filters))

# Get instance_ids from describe_instances_response.
def get_instance_ids(describe_instances_response):
    instance_ids = []
    if describe_instances_response["Reservations"]:
        for reservation in describe_instances_response["Reservations"]:
            instance_ids.extend(instance["InstanceId"] for instance in reservation["Instances"] if instance.get("InstanceId"))
    return instance_ids

WAITER_LOCK = threading.Lock() 
def run_waiter_100_instances_for_status(ec2_client, status, instance_ids):
    time.sleep(10)
    WAITER_LOCK.acquire()
    waiter = ec2_client.get_waiter('instance_running')
    WAITER_LOCK.release()
    waiter.wait(InstanceIds=instance_ids)

def run_waiter_for_status(ec2_client, status, instance_ids):
    thread_pool = []
    i = 0
    while i < len(instance_ids):
        j = i + min(len(instance_ids), i + MAX_INTANCES_FOR_WAITER)
        t = threading.Thread(target=run_waiter_100_instances_for_status, args=(
                            ec2_client, status, instance_ids[i:j]))
        t.start()
        thread_pool.append(t)
        i = i + MAX_INTANCES_FOR_WAITER
    for t in thread_pool:
        t.join()

# used for testing only.
# if __name__ == "__main__":
#     ip_list = collect_public_ips('4', "4-NODE-23-36-01-2018-07-05", "configuration.txt")
#     print ip_list
#     generate_distribution_config(2, 1, ip_list, "config_test.txt")
