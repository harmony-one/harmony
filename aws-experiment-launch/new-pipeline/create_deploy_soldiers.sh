if [ $# -lt 3 ]; then
    echo "Please provide # of instances, # of shards, # of clients"
    exit 1
fi

INSTANCE_NUM=$1
SHARD_NUM=$2
CLIENT_NUM=$3
SLEEP_TIME=10

echo "Creating $INSTANCE_NUM instances at 8 regions"
python create_solider_instances.py --regions 1,2,3,4,5,6,7,8 --instances $INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM


echo "Sleep for $SLEEP_TIME seconds"
sleep $SLEEP_TIME

echo "Rung collecint raw ips"
python collect_public_ips.py --instance_output instance_output.txt

sleep 10
echo "Generate distribution_config"
python generate_distribution_config.py --ip_list_file raw_ip.txt --shard_num $SHARD_NUM --client_num $CLIENT_NUM

aws s3 cp distribution_config.txt s3://unique-bucket-bin/distribution_config.txt
