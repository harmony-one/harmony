import argparse
import base64
import boto3
import datetime
import json
import logging
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

IAM_INSTANCE_PROFILE = 'BenchMarkCodeDeployInstanceProfile'
time_stamp = time.time()
CURRENT_SESSION = datetime.datetime.fromtimestamp(
    time_stamp).strftime('%H-%M-%S-%Y-%m-%d')
NODE_NAME_SUFFIX = "NODE-" + CURRENT_SESSION

def run_one_region_instances(config, region_number, number_of_instances, instance_resource=InstanceResource.ON_DEMAND):
    region_name = config[region_number][utils.REGION_NAME]
    # Create session.
    session = boto3.Session(region_name=region_name)
    # Create a client.
    ec2_client = session.client('ec2')

    if instance_resource == InstanceResource.ON_DEMAND:
        node_name_tag = create_instances(
            config, ec2_client, region_number, int(number_of_instances))
        LOGGER.info("Created %s in region %s" % (node_name_tag, region_number))
        return node_name_tag, ec2_client
    else:
        return None, None


def create_instances(config, ec2_client, region_number, number_of_instances):
    node_name_tag = region_number + "-" + NODE_NAME_SUFFIX
    LOGGER.info("Creating node_name_tag: %s" % node_name_tag)
    available_zone = utils.get_one_availability_zone(ec2_client)
    LOGGER.info("Looking at zone %s to create instances." % available_zone)

    ec2_client.run_instances(
        MinCount=number_of_instances,
        MaxCount=number_of_instances,
        ImageId=config[region_number][utils.REGION_AMI],
        Placement={
            'AvailabilityZone': available_zone,
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

    count = 0
    while count < 40:
        time.sleep(5)
        LOGGER.info("Waiting ...")
        ip_list = utils.collect_public_ips_from_ec2_client(ec2_client, node_name_tag)
        if len(ip_list) == number_of_instances:
            LOGGER.info("Created %d instances" % number_of_instances)
            return node_name_tag
        count = count + 1
    LOGGER.info("Can not create %d instances" % number_of_instances)
    return None

lock = threading.Lock()

def run_for_one_region(config, region_number, number_of_instances, instance_resouce, fout, fout2):
    node_name_tag, ec2_client = run_one_region_instances(config, region_number, number_of_instances, InstanceResource.ON_DEMAND)
    if node_name_tag:
        LOGGER.info("Managed to create instances for region %s" % region_number )
        instance_ids = utils.get_instance_ids2(ec2_client, node_name_tag)
        lock.acquire()
        try:
            fout.write("%s %s\n" % (node_name_tag, region_number))
            for instance_id in instance_ids:
                fout2.write(instance_id + " " + node_name_tag + " " + region_number + " " + config[region_number][utils.REGION_NAME] + "\n")
        finally:
            lock.release()
    else:
        LOGGER.info("Failed to create instances for region %s" % region_number )

if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description='This script helps you start instances across multiple regions')
    parser.add_argument('--regions', type=str, dest='regions',
                        default='3', help="Supply a csv list of all regions")
    parser.add_argument('--instances', type=str, dest='num_instance_list',
                        default='1', help='number of instances in respective of region')
    parser.add_argument('--region_config', type=str, dest='region_config', default='configuration.txt')
    parser.add_argument('--instance_output', type=str, dest='instance_output',
                        default='instance_output.txt', help='the file to append or write')
    parser.add_argument('--instance_ids_output', type=str, dest='instance_ids_output',
                        default='instance_ids_output.txt', help='the file to append or write')
    parser.add_argument('--append', dest='append', type=bool, default=False,
                        help='append to the current instance_output')
    args = parser.parse_args()
    config = utils.read_region_config(args.region_config)
    region_list = args.regions.split(',')
    num_instance_list = args.num_instance_list.split(',')
    assert len(region_list) == len(num_instance_list), "number of regions: %d != number of instances per region: %d" % (len(region_list), len(num_instance_list))

    write_mode = "a" if args.append else "w"
    with open(args.instance_output, write_mode) as fout, open(args.instance_ids_output, write_mode) as fout2:
        thread_pool = []
        for i in range(len(region_list)):
            region_number = region_list[i]
            number_of_instances = num_instance_list[i]
            t = threading.Thread(target=run_for_one_region, args=(config, region_number, number_of_instances, InstanceResource.ON_DEMAND, fout, fout2))
            LOGGER.info("creating thread for region %s" % region_number)
            t.start()
            thread_pool.append(t)
        for t in thread_pool:
            t.join()
        LOGGER.info("done.")
