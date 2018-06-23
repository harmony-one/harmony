# Since `go run` will generate a temporary exe every time,
# your system will pop up a network security dialog for each instance
# and you won't be able to turn it off. With `go build` generating one
# exe, the dialog will only pop up once at the very first time.
# Also it's recommended to use `go build` for testing the whole exe. 
go build # Build the harmony-benchmark.exe
go build aws-code/transaction_generator.go

# Create a tmp folder for logs
t=`date +"%Y%m%d-%H%M%S"`
log_folder="log-$t"

if [ ! -d $log_folder ] 
then
    mkdir -p $log_folder
fi

# Start nodes
config="local_config.txt"
while IFS='' read -r line || [[ -n "$line" ]]; do
    IFS=' ' read ip port mode <<< $line
    # echo $ip $port $mode
    ./harmony-benchmark.exe -ip $ip -port $port -config_file $config -log_folder $log_folder&
done < $config

# Generate transactions
./transaction_generator.exe -config_file $config -log_folder $log_folder