
import utils


def create(config, ec2_client, region_number, instance_type):
    return ec2_client.create_launch_template(
        # DryRun=True,
        LaunchTemplateName='richard-benchmark-' + \
        config[region_number][utils.REGION_NAME] + '-' + instance_type,
        LaunchTemplateData={
            'IamInstanceProfile': {
                'Name': utils.IAM_INSTANCE_PROFILE
            },
            'ImageId': config[region_number][utils.REGION_AMI],
            'InstanceType': instance_type,
            'KeyName':  config[region_number][utils.REGION_KEY],
            'UserData': utils.USER_DATA_BASE64,
            'SecurityGroupIds': [
                config[region_number][utils.REGION_SECURITY_GROUP_ID]
            ],
            # 'InstanceInitiatedShutdownBehavior': 'stop',
            # 'TagSpecifications': [
            #     {
            #         'ResourceType': 'instance',
            #         'Tags': [
            #             {
            #                 'Key': 'Name',
            #                 'Value': 'string'
            #             }
            #         ]
            #     }
            # ],
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
