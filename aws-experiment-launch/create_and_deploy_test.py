import unittest
import datetime
from create_and_deploy import get_public_ips
from create_and_deploy import generate_config_file

class TestCreateAndDeploy(unittest.TestCase):

    def test_get_public_ips(self):
        response = {
            'Groups': [],
            'Instances': [
                {
                    'AmiLaunchIndex': 123,
                    'PublicDnsName': 'string',
                    'PublicIpAddress': 'minhdoan'
                },
                {
                    'AmiLaunchIndex': 123,
                    'PublicDnsName': 'string',
                    'PublicIpAddress': 'harmony'
                },
            ],
            'OwnerId': 'string',
            'RequesterId': 'string',
        }
        ips = get_public_ips(response)
        self.assertEqual(['minhdoan', 'harmony'], ips)

    def test_generate_config_file(self):
        ips = ["102.000.000.1", "102.000.000.2", "102.000.000.3", "102.000.000.4", "102.000.000.5", "102.000.000.6"]
        generate_config_file(2, 2, ips, "config_test.txt")

if __name__ == '__main__':
    unittest.main()