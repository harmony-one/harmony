INSTANCE_NUM=50

echo "Creating $INSTANCE_NUM instances at 8 regions"
python create_instances.py --regions 1,2,3,4,5,6,7,8 --instances $INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM,$INSTANCE_NUM --instance_output instance_output_50.txt
