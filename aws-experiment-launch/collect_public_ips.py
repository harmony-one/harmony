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
    parser.add_argument('--region_config', type=str,
                        dest='region_config', default='configuration.txt')
    parser.add_argument('--file_output', type=str,
                        dest='file_output', default='raw_ip.txt')
    args = parser.parse_args()

    if not args.instance_output or not args.instance_output or not os.path.isfile(args.instance_output):
        print "%s or %s are not existed" % (args.file_output, args.instance_output)
        sys.exit(1)
    if args.instance_output:
        with open(args.instance_output, "r") as fin, open(args.file_output, "w") as fout:
            for line in fin.readlines():
                items = line.split(" ")
                region_number = items[1].strip()
                node_name_tag = items[0].strip()
                ip_list = utils.collect_public_ips(region_number, node_name_tag, args.region_config)
                random.shuffle(ip_list)
                for ip in ip_list:
                    fout.write(ip + " " + node_name_tag + "\n")
        print "Done collecting public ips %s" % args.file_output
