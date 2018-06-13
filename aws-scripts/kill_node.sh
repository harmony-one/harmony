for pid in `/bin/ps -fu $USER| grep "slave.go\|slave -port\|leader\|benchmark_node" | grep -v "grep" | awk '{print $2}'`;
do
    echo 'Killed process: '$pid
    kill -9 $pid
done
