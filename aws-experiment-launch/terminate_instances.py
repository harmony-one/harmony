import argparse
import os
import random
import sys

from utils import utils

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='This script helps you to collect public ips')
    parser.add_argument('--instance_output', type=str, dest='instance_output',
                        default='instance_output.txt',
                        help='the file contains node_name_tag and region number of created instances.')
    parser.add_argument('--node_name_tag', type=str, dest='node_name_tag')
    parser.add_argument('--region_number', type=str, dest='region_number')
    parser.add_argument('--region_config', type=str,
                        dest='region_config', default='configuration.txt')
    args = parser.parse_args()

    if not args.instance_output or not os.path.isfile(args.instance_output):
        print "%s is not existed" % args.instance_output
        sys.exit(1)
    if args.region_number and args.node_name_tag:
        ec2_client, session = utils.create_ec2_client(args.region_number, args.region_config)
        filters = [{'Name': 'tag:Name','Values': [args.node_name_tag]}]
        instance_ids = utils.get_instance_ids(ec2_client.describe_instances(Filters=filters))
        ec2_client.terminate_instances(InstanceIds=instance_ids)
        print "waiting until instances with tag %s died." % args.node_name_tag
        waiter = ec2_client.get_waiter('instance_terminated')
        waiter.wait(InstanceIds=instance_ids)
    elif args.instance_output:
        with open(args.instance_output, "r") as fin:
            for line in fin.readlines():
                items = line.split(" ")
                region_number = items[1].strip()
                node_name_tag = items[0].strip()
                ec2_client, session = utils.create_ec2_client(region_number, args.region_config)
                filters = [{'Name': 'tag:Name','Values': [node_name_tag]}]
                instance_ids = utils.get_instance_ids(ec2_client.describe_instances(Filters=filters))
                ec2_client.terminate_instances(InstanceIds=instance_ids)
                print "waiting until instances with tag %s died." % node_name_tag
                waiter = ec2_client.get_waiter('instance_terminated')
                waiter.wait(InstanceIds=instance_ids)
