import boto3
import datetime
import json
import sys
import time


REGION_NAME = 'region_name'
REGION_KEY = 'region_key'
REGION_SECURITY_GROUP = 'region_security_group'
REGION_HUMAN_NAME = 'region_human_name'
INSTANCE_TYPE = 't2.micro'
REGION_AMI = 'region_ami'

def read_region_config(region_config='configuration.txt'):
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
    return session.client('ec2')

def collect_public_ips(region_number, node_name_tag, region_config):
    ec2_client = create_ec2_client(region_number, region_config)
    filters = [{'Name': 'tag:Name','Values': [node_name_tag]}]
    response = ec2_client.describe_instances(Filters=filters)
    ip_list = []
    for reservation in response[u'Reservations']:
        ip_list.extend(instance['PublicIpAddress'] for instance in reservation['Instances'])
    return ip_list

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
    if len(ip_list) < shard_number * 2 + client_number:
        print("Not enough nodes to generate a config file")
        return False

    # Create ip for clients.
    client_id, leader_id, validator_id = 0, 0, 0
    with open(distribution_config, "w") as fout:
        for i in range(len(ip_list)):
            if client_id < client_number:
                fout.write("%s 9000 client %d\n" % (ip_list[i], client_id % shard_number))
                client_id = client_id + 1
            elif leader_id < shard_number:
                fout.write("%s 9000 leader %d\n" % (ip_list[i], leader_id))
                leader_id = leader_id + 1
            else:
                fout.write("%s 9000 validator %d\n" % (ip_list[i], validator_id % shard_number))
                validator_id = validator_id + 1

def get_availability_zones(ec2_client):
    response = ec2_client.describe_availability_zones()
    all_zones = []
    if response.get('AvailabilityZones', None):
        all_zones = [info['ZoneName'] for info in response.get('AvailabilityZones') if info['State'] == 'available']
    return all_zones

def get_one_availability_zone(ec2_client):
    all_zones = get_availability_zones(ec2_client)
    if len(all_zones) > 0:
        return all_zones[0]
    else:
        return None

# used for testing only.
# if __name__ == "__main__":
#     ip_list = collect_public_ips('4', "4-NODE-23-36-01-2018-07-05", "configuration.txt")
#     print ip_list
#     generate_distribution_config(2, 1, ip_list, "config_test.txt")
