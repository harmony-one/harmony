./kill_node.sh
ipfile=$1
while read ip port mode; do 
	#echo $ip $port $mode $ipfile
  go run ./benchmark_main.go -ip $ip -port $port -ipfile $ipfile&
done < $ipfile