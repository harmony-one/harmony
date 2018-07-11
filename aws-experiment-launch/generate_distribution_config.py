import argparse
import logging
import sys

from utils import utils

logging.basicConfig(level=logging.INFO, format='%(threadName)s %(asctime)s - %(name)s - %(levelname)s - %(message)s')
LOGGER = logging.getLogger(__file__)
LOGGER.setLevel(logging.INFO)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='This script helps you to genereate distribution config')
    parser.add_argument('--ip_list_file', type=str, dest='ip_list_file',
                        default='raw_ip.txt', help="the file containing available raw ips")
    # If the ip_list_file is None we need to use the region, node_name_tag and region_config to collect raw_ip                        
    parser.add_argument('--region', type=str, dest='region_number',
                        default="4", help="region number")
    parser.add_argument('--node_name_tag', type=str,
                        dest='node_name_tag', default='4-NODE-23-36-01-2018-07-05')
    parser.add_argument('--region_config', type=str,
                        dest='region_config', default='configuration.txt')

    parser.add_argument('--shard_number', type=int, dest='shard_number', default=1)
    parser.add_argument('--client_number', type=int, dest='client_number', default=1)
    parser.add_argument('--distribution_config', type=str,
                        dest='distribution_config', default='distribution_config.txt')
    args = parser.parse_args()

    if args.ip_list_file == None:
        utils.generate_distribution_config2(
                args.region_number, args.node_name_tag, args.region_config,
                args.shard_number, args.client_number, args.distribution_config)
    else:
        utils.generate_distribution_config3(args.shard_number, args.client_number,
                                            args.ip_list_file, args.distribution_config)
    LOGGER.info("Done writing %s" % args.distribution_config)
