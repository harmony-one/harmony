import argparse
import logging
import os
import random
import sys
import threading
import boto3



logging.basicConfig(level=logging.INFO, format='%(threadName)s %(asctime)s - %(name)s - %(levelname)s - %(message)s')
LOGGER = logging.getLogger(__file__)
LOGGER.setLevel(logging.INFO)

REGION_NAME = 'region_name'
REGION_KEY = 'region_key'
REGION_SECURITY_GROUP = 'region_security_group'
REGION_SECURITY_GROUP_ID = 'region_security_group_id'
REGION_HUMAN_NAME = 'region_human_name'
INSTANCE_TYPE = 't2.micro'
REGION_AMI = 'region_ami'

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
            config[region_num][REGION_SECURITY_GROUP_ID] = mylist[6]
    return config

def get_instance_ids(describe_instances_response):
    instance_ids = []
    if describe_instances_response["Reservations"]:
        for reservation in describe_instances_response["Reservations"]:
            instance_ids.extend(instance["InstanceId"] for instance in reservation["Instances"] if instance.get("InstanceId"))
    return instance_ids
    
def get_instance_ids2(ec2_client, node_name_tag):
    time.sleep(5)
    filters = [{'Name': 'tag:Name','Values': [node_name_tag]}]
    return get_instance_ids(ec2_client.describe_instances(Filters=filters))

def create_ec2_client(region_number, region_config):
    config = read_configuration_file(region_config)
    region_name = config[region_number][REGION_NAME]
    session = boto3.Session(region_name=region_name)
    return session.client('ec2'), session

def terminate_instances_by_region(region_number, region_config, node_name_tag):
    ec2_client, _ = create_ec2_client(region_number, region_config)
    filters = [{'Name': 'tag:Name','Values': [node_name_tag]}]
    instance_ids = get_instance_ids(ec2_client.describe_instances(Filters=filters))
    if instance_ids:
        ec2_client.terminate_instances(InstanceIds=instance_ids)
        LOGGER.info("waiting until instances with tag %s died." % node_name_tag)
        waiter = ec2_client.get_waiter('instance_terminated')
        waiter.wait(InstanceIds=instance_ids)
        LOGGER.info("instances with node name tag %s terminated." % node_name_tag)
    else:
        pass
        LOGGER.warn("there is no instances to terminate")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='This script helps you to collect public ips')
    parser.add_argument('--instance_output', type=str, dest='instance_output',
                        default='instance_output.txt',
                        help='the file contains node_name_tag and region number of created instances.')
    parser.add_argument('--node_name_tag', type=str, dest='node_name_tag')
    parser.add_argument('--region_config', type=str,
                        dest='region_config', default='configuration.txt')
    args = parser.parse_args()

    if args.node_name_tag:
        node_name_tag_items = args.node_name_tag.split(",")
        region_number_items = [item[:1] for item in node_name_tag_items]
        thread_pool = []
        for i in range(len(region_number_items)):
            region_number = region_number_items[i]
            node_name_tag = node_name_tag_items[i]
            t = threading.Thread(target=terminate_instances_by_region, args=(region_number, args.region_config, node_name_tag))
            t.start()
            thread_pool.append(t)
        for t in thread_pool:
            t.join()
        LOGGER.info("done.")
    elif args.instance_output:
        with open(args.instance_output, "r") as fin:
            thread_pool = []
            for line in fin.readlines():
                items = line.split(" ")
                region_number = items[1].strip()
                node_name_tag = items[0].strip()
                t = threading.Thread(target=terminate_instances_by_region, args=(region_number, args.region_config, node_name_tag))
                t.start()
                thread_pool.append(t)
            for t in thread_pool:
                t.join()
            LOGGER.info("done.")
