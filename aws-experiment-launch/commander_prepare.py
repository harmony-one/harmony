import argparse
import logging
import os
import stat
import sys

from utils import utils

logging.basicConfig(level=logging.INFO, format='%(threadName)s %(asctime)s - %(name)s - %(levelname)s - %(message)s')
LOGGER = logging.getLogger(__file__)
LOGGER.setLevel(logging.INFO)

PEMS = [
    "virginia-key-benchmark.pem",
    "ohio-key-benchmark.pem",
    "california-key-benchmark.pem",
    "oregon-key-benchmark.pem",
    "tokyo-key-benchmark.pem",
    "singapore-key-benchmark.pem",
    "frankfurt-key-benchmark.pem",
    "ireland-key-benchmark.pem",
]

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='This script helps you to genereate distribution config')
    parser.add_argument('--distribution_config', type=str,
                        dest='distribution_config', default='distribution_config.txt')
    parser.add_argument('--commander_logging', type=str,
                        dest='commander_logging', default='commander_logging.sh')
    parser.add_argument('--logs_download', type=str,
                        dest='logs_download', default='logs_download.sh')
    parser.add_argument('--commander_info', type=str,
                        dest='commander_info', default='commander_info.txt')

    args = parser.parse_args()

    if not os.path.exists(args.distribution_config):
        sys.exit(1)
    with open(args.distribution_config, "r") as fin:
        lines = fin.readlines()

    commander_address = None
    commander_region = None
    commander_output = None
    with open(args.distribution_config, "w") as fout:
        for line in lines:
            if "commander" in line:
                items = [item.strip() for item in line.split(" ")]
                commander_address = items[0]
                commander_region = int(items[4][0])
                commander_output = "\n".join(items)
            else:
                fout.write(line.strip() + "\n")
    if not commander_address or not commander_region:
        LOGGER.info("Failed to extract commander address and commander region.")
        sys.exit(1)

    with open(args.commander_info, "w") as fout:
        fout.write(commander_output)

    LOGGER.info("Generated %s" % args.distribution_config)
    LOGGER.info("Generated %s" % args.commander_info)
    with open(args.commander_logging, "w") as fout:
        fout.write("scp -i ../keys/%s %s ec2-user@%s:/tmp/distribution_config.txt\n" % (PEMS[commander_region - 1], args.distribution_config, commander_address))
        fout.write("scp -i ../keys/%s %s ec2-user@%s:/tmp/commander_info.txt\n" % (PEMS[commander_region - 1], args.commander_info, commander_address))
        fout.write("if [ $? -eq 0 ]; then\n\t")
        fout.write("ssh -i ../keys/%s ec2-user@%s\n" % (PEMS[commander_region - 1], commander_address))
        fout.write("else\n\techo \"Failed to send %s to the commander machine\"\nfi\n" % args.distribution_config)
    st = os.stat(args.commander_logging)
    os.chmod(args.commander_logging, st.st_mode | stat.S_IEXEC)
    LOGGER.info("Generated %s" % args.commander_logging)

    with open(args.logs_download, "w") as fout:
        fout.write("scp -i ../keys/%s ec2-user@%s:~/projects/src/harmony-benchmark/bin/upload tmp/\n" % (PEMS[commander_region - 1], args.distribution_config, commander_address))
    st = os.stat(args.logs_download)
    os.chmod(args.logs_download, st.st_mode | stat.S_IEXEC)
    LOGGER.info("Generated %s" % args.logs_download)

    LOGGER.info("DONE.")

