import utils
import logger
LOGGER = logger.getLogger(__file__)

def create_launch_specification(config, region_number, instanceType):
    return {
        # Region irrelevant fields
        'IamInstanceProfile': {
            'Name': utils.IAM_INSTANCE_PROFILE
        },
        'InstanceType': instanceType,
        'UserData': utils.USER_DATA_BASE64,
        # Region relevant fields
        'SecurityGroups': [
            {
                # In certain scenarios, we have to use group id instead of group name
                # https://github.com/boto/boto/issues/350#issuecomment-27359492
                'GroupId': config[region_number][utils.REGION_SECURITY_GROUP_ID]
            }
        ],
        'ImageId': config[region_number][utils.REGION_AMI],
        'KeyName': config[region_number][utils.REGION_KEY],
        'TagSpecifications': [
            {
                'ResourceType': 'instance',
                'Tags': [
                    {
                        'Key': 'Name',
                        'Value': utils.get_node_name_tag(region_number)
                    }
                ]
            }
        ],
        # 'WeightedCapacity': 123.0,
        # 'Placement': {
        #     # 'AvailabilityZone': get_one_availability_zone(ec2_client)
        # }
    }

def create_launch_specification_list(config, region_number, instance_type_list):
    return list(map(lambda type: create_launch_specification(config, region_number, type), instance_type_list))

def request_spot_fleet(config, ec2_client, region_number, number_of_instances, instance_type_list):
    LOGGER.info("Requesting spot fleet")
    LOGGER.info("Creating node_name_tag: %s" % utils.get_node_name_tag(region_number))
    # https://boto3.readthedocs.io/en/latest/reference/services/ec2.html#EC2.Client.request_spot_fleet
    response = ec2_client.request_spot_fleet(
        # DryRun=True,
        SpotFleetRequestConfig={
            # https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-fleet.html#spot-fleet-allocation-strategy
            'AllocationStrategy': 'diversified',
            'IamFleetRole': 'arn:aws:iam::656503231766:role/RichardFleetRole',
            'LaunchSpecifications': create_launch_specification_list(config, region_number, instance_type_list),
            # 'SpotPrice': 'string', # The maximum price per unit hour that you are willing to pay for a Spot Instance. The default is the On-Demand price.
            'TargetCapacity': number_of_instances,
            'OnDemandTargetCapacity': 0,
            'Type': 'maintain'
        }
    )
    return response