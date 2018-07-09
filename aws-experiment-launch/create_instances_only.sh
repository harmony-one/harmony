INSTANCE_NUM=1

echo "Creating $$INSTANCE_NUM instances at 8 regions"
python create_instances.py --regions 1,2,3,4,5,6,7,8 --instances $INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM
