import argparse
import sys

from utils import utils

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='This script helps you to collect public ips')
    parser.add_argument('--region', type=str, dest='region_number',
                        default="4", help="region number")
    parser.add_argument('--node_name_tag', type=str,
                        dest='node_name_tag', default='4-NODE-23-36-01-2018-07-05')
    parser.add_argument('--region_config', type=str,
                        dest='region_config', default='configuration.txt')
    parser.add_argument('--file_output', type=str,
                        dest='file_output', default='raw_ip.txt')
    args = parser.parse_args()

    ip_list = utils.collect_public_ips(args.region_number, args.node_name_tag, args.region_config)
    with open(args.file_output, "w") as fout:
        for ip in ip_list:
            fout.write(ip + "\n")

