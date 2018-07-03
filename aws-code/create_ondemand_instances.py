import os
import argparse
import json
import time
import datetime

REGION_NAME = 'region_name'
REGION_KEY = 'region_key'
REGION_SECURITY_GROUP = 'region_security_group'
REGION_HUMAN_NAME = 'region_human_name'
INSTANCE_TYPE = 't2.micro'
AMI = 'ami-a9d09ed1'
USER_DATA = 'user-data.sh'
IAM_INSTANCE_PROFILE = 'BenchMarkCodeDeployInstanceProfile'

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
    return config

def create_custom_json(config,num_instances,region_num,current_session):
    print(num_instances)
    input_cli = {}
    input_cli['MinCount'] = num_instances
    input_cli['MaxCount'] = num_instances
    input_cli['ImageId'] = 'ami-b70554c8'
    input_cli['Placement'] = {}
    input_cli['Placement']['AvailabilityZone'] = config[region_num][REGION_NAME] +"a"
    input_cli['SecurityGroups'] = []
    input_cli['SecurityGroups'].append(config[region_num][REGION_SECURITY_GROUP])
    input_cli['IamInstanceProfile'] = {}
    input_cli['IamInstanceProfile']['Name'] = IAM_INSTANCE_PROFILE
    input_cli['KeyName'] = config[region_num][REGION_KEY]
    #input_cli['KeyName'] = "main"
    input_cli['UserData'] = USER_DATA
    input_cli['InstanceType'] = INSTANCE_TYPE
    input_cli['TagSpecifications'] = []
    input_cli['TagSpecifications'].append({"ResourceType": "instance","Tags":[{"Key":"Name","Value":"Node"}]})
    my_dir = "input_jsons/" + "session-"+ current_session 
    if not os.path.exists(my_dir):
        os.makedirs(my_dir)
    cli_input_file = os.path.join(my_dir,config[region_num][REGION_HUMAN_NAME]+".json")
    cli_input_file = "local.json"
    with open(cli_input_file,'w') as g:
        json.dump(input_cli,g)
    print("INPUT CLI JSON FILE: %s" % cli_input_file)
    return cli_input_file

def create_instances(region_list,instances_list,current_session):
    for i in range(len(region_list)):
        region_num = region_list[i]
        num_instances = int(instances_list[i])
        cli_input_file = create_custom_json(config,num_instances,region_num,current_session)
        
if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='This script helps you start instances across multiple regions')
    parser.add_argument('--regions',type=str,dest='regions',default='3',help="Supply a csv list of all regions")
    parser.add_argument('--instances', type=str,dest='numInstances',default=1,help='number of shards')
    parser.add_argument('--configuration',type=str,dest='config',default='configuration.txt')
    args = parser.parse_args()
    config = read_configuration_file(args.config)
    region_list = args.regions.split(',')
    instances_list = args.numInstances.split(',')
    assert len(region_list) == len(instances_list),"number of regions: %d != number of instances per region: %d" % (len(region_list),len(intances_list))
    time_stamp = time.time()
    current_session = datetime.datetime.fromtimestamp(time_stamp).strftime('%H-%M-%S-%Y-%m-%d')
    print("current session is %s" % current_session)
    create_instances(region_list,instances_list,current_session)
    
	