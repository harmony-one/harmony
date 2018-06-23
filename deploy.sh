# Create a tmp folder for logs
t=`date +"%Y%m%d-%H%M%S"`
log_folder="log-$t"

if [ ! -d $log_folder ] 
then
    mkdir -p $log_folder
fi

./kill_node.sh
config=$1
while IFS='' read -r line || [[ -n "$line" ]]; do
  IFS=' ' read ip port mode <<< $line
	#echo $ip $port $mode $config
  go run ./benchmark_main.go -ip $ip -port $port -config_file $config -log_folder $log_folder&
done < $config

go run ./aws-code/transaction_generator.go -config_file $config -log_folder $log_folder