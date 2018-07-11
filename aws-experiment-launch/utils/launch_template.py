
import utils


def get_launch_template_name(config, region_number):
    return 'benchmark-' + config[region_number][utils.REGION_NAME]


def create(config, ec2_client, region_number):
    return ec2_client.create_launch_template(
        # DryRun=True,
        LaunchTemplateName=get_launch_template_name(config, region_number),
        LaunchTemplateData={
            'IamInstanceProfile': {
                'Name': utils.IAM_INSTANCE_PROFILE
            },
            'ImageId': config[region_number][utils.REGION_AMI],
            # 'InstanceType': instance_type,
            'KeyName':  config[region_number][utils.REGION_KEY],
            'UserData': utils.USER_DATA_BASE64,
            'SecurityGroupIds': [
                config[region_number][utils.REGION_SECURITY_GROUP_ID]
            ],
            # 'InstanceInitiatedShutdownBehavior': 'stop',
            'TagSpecifications': [
                {
                    'ResourceType': 'instance',
                    'Tags': [
                        {
                            'Key': 'LaunchTemplate',
                            'Value': 'Yes'
                        }
                    ]
                }
            ],
            # 'InstanceMarketOptions': {
            #     'MarketType': 'spot',
            #     'SpotOptions': {
            #         'MaxPrice': 'string',
            #         'SpotInstanceType': 'one-time'|'persistent',
            #         'BlockDurationMinutes': 123,
            #         'InstanceInterruptionBehavior': 'hibernate'|'stop'|'terminate'
            #     }
            # },
        }
    )