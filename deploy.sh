./kill_node.sh
ipfile=$1
while IFS='' read -r line || [[ -n "$line" ]]; do
  IFS=' ' read ip port mode <<< $line
	#echo $ip $port $mode $ipfile
  go run ./benchmark_main.go -ip $ip -port $port -ipfile $ipfile&
done < $ipfile

go run ./aws-code/transaction_generator.go