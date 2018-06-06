
ipfile=$1
while read ip port mode; do 
	echo $ip $port $mode $ipfile
  go run ./benchmark_node.go -ip $ip -port $port -mode $mode  -ipfile $ipfile&
done < $ipfile