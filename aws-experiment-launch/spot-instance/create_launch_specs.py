import os
import argparse
import json
import time
import datetime

REGION_NAME = 'region_name'
REGION_KEY = 'region_key'
REGION_SECURITY_GROUP = 'region_security_group'
REGION_HUMAN_NAME = 'region_human_name'

INSTANCE_TYPE = 'm3.medium' # 't2.micro'
AMI = 'ami-f2d3638a' # 'ami-a9d09ed1'
# base64 userdata.sh
USER_DATA = 'IyEvYmluL2Jhc2gKUkVHSU9OPSQoY3VybCAxNjkuMjU0LjE2OS4yNTQvbGF0ZXN0L21ldGEtZGF0YS9wbGFjZW1lbnQvYXZhaWxhYmlsaXR5LXpvbmUvIHwgc2VkICdzL1thLXpdJC8vJykKeXVtIC15IHVwZGF0ZQp5dW0gaW5zdGFsbCAteSBydWJ5CmNkIC9ob21lL2VjMi11c2VyCmN1cmwgLU8gaHR0cHM6Ly9hd3MtY29kZWRlcGxveS0kUkVHSU9OLnMzLmFtYXpvbmF3cy5jb20vbGF0ZXN0L2luc3RhbGwKY2htb2QgK3ggLi9pbnN0YWxsCi4vaW5zdGFsbCBhdXRv'
IAM_INSTANCE_PROFILE = 'BenchMarkCodeDeployInstanceProfile'

def read_configuration_file(filename):
    config = {}
    with open(filename,'r') as f:
        for line in f:
            vals = line.strip().split(',')
            region_num = vals[0]
            config[region_num] = {}
            config[region_num][REGION_NAME] = vals[1]
            config[region_num][REGION_KEY] = vals[2]
            config[region_num][REGION_SECURITY_GROUP] = vals[3]
            config[region_num][REGION_HUMAN_NAME] = vals[4]
    return config

def create_launch_specification(region_num):
    input_cli = {}
    input_cli['ImageId'] = AMI
    # input_cli['Placement'] = {
    #     "AvailabilityZone": config[region_num][REGION_NAME] +"a"
    # }
    input_cli['SecurityGroups'] = [ "richard-spot-instance SSH" ] # [ config[region_num][REGION_SECURITY_GROUP] ]
    input_cli['IamInstanceProfile'] = {
        "Name": IAM_INSTANCE_PROFILE
    }
    input_cli['KeyName'] = "richard-spot-instance" # config[region_num][REGION_KEY]
    input_cli['UserData'] = USER_DATA
    input_cli['InstanceType'] = INSTANCE_TYPE
    # folder = "launch_specs/" + "session-"+ current_session 
    folder = "launch_specs/latest"
    if not os.path.exists(folder):
        os.makedirs(folder)
    launch_spec_file = os.path.join(folder,config[region_num][REGION_HUMAN_NAME]+".json")
    with open(launch_spec_file,'w') as g:
        json.dump(input_cli,g)
    print("Launch spec: %s" % launch_spec_file)
    return launch_spec_file

def create_instances(region_list):
    for i in range(len(region_list)):
        region_num = region_list[i]
        launch_spec_file = create_launch_specification(region_num)
        
if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='This script helps you start instances across multiple regions')
    parser.add_argument('--regions',type=str,dest='regions',default='3',help="a comma-separated-value list of all regions")
    # configuration file contains the number/name/security-group etc. information of each region.
    parser.add_argument('--config',type=str,dest='config',default='configuration.txt')
    args = parser.parse_args()
    config = read_configuration_file(args.config)
    region_list = args.regions.split(',')
    time_stamp = time.time()
    current_session = datetime.datetime.fromtimestamp(time_stamp).strftime('%Y-%m-%d-%H-%M-%S')
    print("current session is %s" % current_session)
    create_instances(region_list)