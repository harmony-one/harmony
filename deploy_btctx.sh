# Kill nodes if any
./kill_node.sh

# Since `go run` will generate a temporary exe every time,
# On windows, your system will pop up a network security dialog for each instance
# and you won't be able to turn it off. With `go build` generating one
# exe, the dialog will only pop up once at the very first time.
# Also it's recommended to use `go build` for testing the whole exe. 
go build -o bin/benchmark
go build -o bin/btctxgen client/btctxgen/main.go

# Create a tmp folder for logs
t=`date +"%Y%m%d-%H%M%S"`
log_folder="tmp_log/log-$t"

mkdir -p $log_folder

# Start nodes
config=$1
while IFS='' read -r line || [[ -n "$line" ]]; do
  IFS=' ' read ip port mode shardId <<< $line
	#echo $ip $port $mode
  if [ "$mode" != "client" ]; then
    ./bin/benchmark -ip $ip -port $port -config_file $config -log_folder $log_folder&
  fi
done < $config

# Generate transactions
./bin/btctxgen -config_file $config -log_folder $log_folder