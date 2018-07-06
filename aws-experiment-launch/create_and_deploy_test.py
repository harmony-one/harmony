import unittest
import datetime
from create_and_deploy import get_public_ips

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

if __name__ == '__main__':
    unittest.main()