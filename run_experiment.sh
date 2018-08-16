# Kill nodes if any
./kill_node.sh

go build -o bin/benchmark
go build -o bin/txgen client/txgen/main.go
go build -o bin/commander aws-experiment-launch/experiment/commander/main.go
go build -o bin/soldier aws-experiment-launch/experiment/soldier/main.go
cd bin

# Create a tmp folder for logs
t=`date +"%Y%m%d-%H%M%S"`
log_folder="tmp_log/log-$t"

mkdir -p $log_folder

# For each of the nodes, start soldier
config=distribution_config.txt
while IFS='' read -r line || [[ -n "$line" ]]; do
  IFS=' ' read ip port mode shardId <<< $line
	#echo $ip $port $mode
  ./soldier -ip $ip -port $port&
done < $config

./commander