import sys
import boto3

REGION_NAME = 'region_name'
REGION_KEY = 'region_key'
REGION_SECURITY_GROUP = 'region_security_group'
REGION_HUMAN_NAME = 'region_human_name'
INSTANCE_TYPE = 't2.small'
REGION_AMI = 'region_ami'

def read_configuration_file(filename='configuration.txt'):
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

def collect_public_ips(region_number, node_name_tag, config):
    config = read_configuration_file(config)
    region_name = config[region_number][REGION_NAME]
    session = boto3.Session(region_name=region_name)
    ec2_client = session.client('ec2')
    filters = [{'Name': 'tag:Name','Values': [node_name_tag]}]
    response = ec2_client.describe_instances(Filters=filters)
    ip_list = []
    for reservation in response[u'Reservations']:
        ip_list.extend(instance['PublicIpAddress'] for instance in reservation['Instances'])
    return ip_list

if __name__ == "__main__":
    collect_public_ips('4', "4-NODE-14-46-47-2018-07-05", "configuration.txt")
