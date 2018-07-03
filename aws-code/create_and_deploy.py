import boto3
import argparse
import sys
import json
import time
import datetime

REGION_NAME = 'region_name'
REGION_KEY = 'region_key'
REGION_SECURITY_GROUP = 'region_security_group'
REGION_HUMAN_NAME = 'region_human_name'
INSTANCE_TYPE = 't2.micro'
REGION_AMI = 'region_ami'
USER_DATA = 'user-data.sh'
IAM_INSTANCE_PROFILE = 'BenchMarkCodeDeployInstanceProfile'

def run_one_region(config,region_number,number_of_instances):
    region_name = config[region_number][REGION_NAME]
    session = boto3.Session(region_name=region_name)
    ec2_client = session.client('ec2')
    create_instances(config,ec2_client,region_number,int(number_of_instances))
    
def get_availability_zones(ec2_client):
    response = ec2_client.describe_availability_zones()
    all_zones = []
    if response.get('AvailabilityZones',None):
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

def create_instances(config,ec2_client,region_number,number_of_instances):
    response = ec2_client.run_instances(
        MinCount = number_of_instances,
        MaxCount = number_of_instances,
        ImageId = config[region_number][REGION_AMI],
        Placement = {
        'AvailabilityZone': get_one_availability_zone(ec2_client)
        },
        SecurityGroups = [config[region_number][REGION_SECURITY_GROUP]],
        IamInstanceProfile = {
            'Name' : IAM_INSTANCE_PROFILE
        },
        KeyName = config[region_number][REGION_KEY],
        UserData = USER_DATA,
        InstanceType = INSTANCE_TYPE,
        TagSpecifications = [
          {
            'ResourceType' : 'instance',
            'Tags': [
                {
                    'Key': 'Name',
                    'Value': 'Node'
                },
            ]
          },
        ]
    )
    print(response)
    return response

def read_configuration_file(filename):
    config = {}
    with open(filename,'r') as f:
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

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='This script helps you start instances across multiple regions')
    parser.add_argument('--regions',type=str,dest='regions',default='3',help="Supply a csv list of all regions")
    parser.add_argument('--instances', type=str,dest='numInstances',default='1',help='number of instances')
    parser.add_argument('--configuration',type=str,dest='config',default='configuration.txt')
    #parser.add_argument('--availability',type=str,dest='availability',default='availability-zones.txt')
    args = parser.parse_args()
    config = read_configuration_file(args.config)
    #availability_zone = read_availability_zone(args.availability)
    region_list = args.regions.split(',')
    instances_list = args.numInstances.split(',')
    assert len(region_list) == len(instances_list),"number of regions: %d != number of instances per region: %d" % (len(region_list),len(intances_list))
    time_stamp = time.time()
    current_session = datetime.datetime.fromtimestamp(time_stamp).strftime('%H-%M-%S-%Y-%m-%d')
    print("current session is %s" % current_session)
    region_number  = '1'
    number_of_instances = '2'
    print run_one_region(config,region_number,number_of_instances)