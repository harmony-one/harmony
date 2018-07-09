INSTANCE_NUM=1
SHARD_NUM=1
CLIENT_NUM=1

echo "Creating $$INSTANCE_NUM instances at 8 regions"
python create_instances.py --regions 1,2,3,4,5,6,7,8 --instances $INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM

echo "Rung collecint raw ips"
python collect_public_ips.py --instance_output instance_output.txt

echo "Rung collecint raw ips"
python collect_public_ips.py --instance_output instance_output.txt

echo "Rung collecint raw ips"
python generate_distribution_config.py --ip_list_file raw_ip.txt --shard_num $SHARD_NUM --client_num $CLIENT_NUM

echo "Deploy"
python deploy.py

