import utils
import logger
import launch_template
LOGGER = logger.getLogger(__file__)


def create_launch_specification(region_number, instanceType):
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
                'GroupId': utils.CONFIG[region_number][utils.REGION_SECURITY_GROUP_ID]
            }
        ],
        'ImageId': utils.CONFIG[region_number][utils.REGION_AMI],
        'KeyName': utils.CONFIG[region_number][utils.REGION_KEY],
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


def create_launch_specification_list(region_number, instance_type_list):
    return list(map(lambda type: create_launch_specification(region_number, type), instance_type_list))


def get_launch_template(region_number, instance_type):
    return {
        'LaunchTemplateSpecification': {
            'LaunchTemplateName': launch_template.get_launch_template_name(region_number),
            'Version': '1'
        },
        'Overrides': [
            {
                'InstanceType': instance_type
            }
        ]
    }


def get_launch_template_list(region_number, instance_type_list):
    return list(map(lambda type: get_launch_template(region_number, type), instance_type_list))


def request_spot_fleet(ec2_client, region_number, number_of_instances, instance_type_list):
    LOGGER.info("Requesting spot fleet")
    LOGGER.info("Creating node_name_tag: %s" %
                utils.get_node_name_tag(region_number))
    # https://boto3.readthedocs.io/en/latest/reference/services/ec2.html#EC2.Client.request_spot_fleet
    response = ec2_client.request_spot_fleet(
        # DryRun=True,
        SpotFleetRequestConfig={
            # https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-fleet.html#spot-fleet-allocation-strategy
            'AllocationStrategy': 'diversified',
            'IamFleetRole': 'arn:aws:iam::656503231766:role/RichardFleetRole',
            'LaunchSpecifications': create_launch_specification_list(region_number, instance_type_list),
            # 'SpotPrice': 'string', # The maximum price per unit hour that you are willing to pay for a Spot Instance. The default is the On-Demand price.
            'TargetCapacity': number_of_instances,
            'Type': 'maintain'
        }
    )
    return response["SpotFleetRequestId"]


def request_spot_fleet_with_on_demand(ec2_client, region_number, number_of_instances, number_of_on_demand, instance_type_list):
    LOGGER.info("Requesting spot fleet")
    LOGGER.info("Creating node_name_tag: %s" %
                utils.get_node_name_tag(region_number))
    # https://boto3.readthedocs.io/en/latest/reference/services/ec2.html#EC2.Client.request_spot_fleet
    response = ec2_client.request_spot_fleet(
        # DryRun=True,
        SpotFleetRequestConfig={
            # https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-fleet.html#spot-fleet-allocation-strategy
            'AllocationStrategy': 'diversified',
            'IamFleetRole': 'arn:aws:iam::656503231766:role/RichardFleetRole',
            'LaunchTemplateConfigs': get_launch_template_list(region_number, instance_type_list),
            # 'SpotPrice': 'string', # The maximum price per unit hour that you are willing to pay for a Spot Instance. The default is the On-Demand price.
            'TargetCapacity': number_of_instances,
            'OnDemandTargetCapacity': number_of_on_demand,
            'Type': 'maintain'
        }
    )
    return response

def get_instance_ids(client, request_id):
    res = client.describe_spot_fleet_instances(
        SpotFleetRequestId=request_id
    )
    return [ inst["InstanceId"] for inst in res["ActiveInstances"] ]

def run_one_region(region_number, number_of_instances, fout, fout2):
    client = utils.create_client(utils.CONFIG, region_number)
    instance_type_list = ['t2.micro', 't2.small', 'm3.medium']
    # node_name_tag = request_spot_fleet_with_on_demand(
    #     client, region_number, int(number_of_instances), 1, instance_type_list)
    request_id = request_spot_fleet(
        client, region_number, int(number_of_instances), instance_type_list)
    instance_ids = get_instance_ids(client, request_id)
    print(instance_ids) # TODO@ricl, no data here since the request is not fulfilled.