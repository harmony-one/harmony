for pid in `/bin/ps -fu $USER| grep "benchmark\|txgen\|soldier\|commander\|profiler" | grep -v "grep" | awk '{print $2}'`;
do
    echo 'Killed process: '$pid
    kill -9 $pid
done
