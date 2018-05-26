for pid in `/bin/ps -fu $USER| grep "slave.go\|slave -port\|leader" | grep -v "grep" | awk '{print $2}'`;
do
    kill -9 $pid
done
