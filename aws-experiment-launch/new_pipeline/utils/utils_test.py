import unittest

from utils import generate_distribution_config

class TestCreateAndDeploy(unittest.TestCase):

    def test_generate_config_file(self):
        ips = ["102.000.000.1", "102.000.000.2", "102.000.000.3", "102.000.000.4", "102.000.000.5", "102.000.000.6"]
        generate_distribution_config(2, 2, ips, "config_test.txt")
        with open("config_test.txt", "r") as fin:
            lines = fin.readlines()
            collection = {}
            collection['ip'] = []
            collection['client'] = {}
            leader_count, validator_count, client_count = 0, 0, 0
            for line in lines:
                strs = line.split(" ")
                assert(not strs[0] in collection['ip'])
                collection['ip'].append(strs[0])
                if strs[2] == "client":
                    client_count = client_count + 1
                elif strs[2] == "leader":
                    leader_count = leader_count + 1
                elif strs[2] == "validator":
                    validator_count = validator_count + 1
            assert(validator_count == 2)
            assert(leader_count == 2)
            assert(client_count == 2)

if __name__ == '__main__':
    unittest.main()