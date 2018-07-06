import sys
import boto3

REGION_NAME = 'region_name'
REGION_KEY = 'region_key'
REGION_SECURITY_GROUP = 'region_security_group'
REGION_HUMAN_NAME = 'region_human_name'
INSTANCE_TYPE = 't2.micro'
REGION_AMI = 'region_ami'

def read_configuration_file(region_config='configuration.txt'):
    config = {}
    with open(region_config, 'r') as f:
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

def collect_public_ips(region_number, node_name_tag, region_config):
    config = read_configuration_file(region_config)
    region_name = config[region_number][REGION_NAME]
    session = boto3.Session(region_name=region_name)
    ec2_client = session.client('ec2')
    filters = [{'Name': 'tag:Name','Values': [node_name_tag]}]
    response = ec2_client.describe_instances(Filters=filters)
    ip_list = []
    for reservation in response[u'Reservations']:
        ip_list.extend(instance['PublicIpAddress'] for instance in reservation['Instances'])
    return ip_list

def generate_distribution_config2(region_number, node_name_tag, region_config, shard_num, client_num, config_filename):
    ip_list = collect_public_ips(region_number, node_name_tag, region_config)
    generate_distribution_config(shard_num, client_num, ip_list, config_filename)

def generate_distribution_config(shard_num, client_num, ip_list, config_filename):
    if len(ip_list) < shard_num * 2 + client_num:
        print("Not enough nodes to generate a config file")
        return False

    # Create ip for clients.
    client_id, leader_id, validator_id = 0, 0, 0
    with open(config_filename, "w") as fout:
        for i in range(len(ip_list)):
            if client_id < client_num:
                fout.write("%s 9000 client %d\n" % (ip_list[i], client_id % shard_num))
                client_id = client_id + 1
            elif leader_id < shard_num:
                fout.write("%s 9000 leader %d\n" % (ip_list[i], leader_id))
                leader_id = leader_id + 1
            else:
                fout.write("%s 9000 validator %d\n" % (ip_list[i], validator_id % shard_num))
                validator_id = validator_id + 1

# used for testing only.
# if __name__ == "__main__":
#     ip_list = collect_public_ips('4', "4-NODE-23-36-01-2018-07-05", "configuration.txt")
#     print ip_list
#     generate_distribution_config(2, 1, ip_list, "config_test.txt")
