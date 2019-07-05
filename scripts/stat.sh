#!/usr/bin/env bash

# stat.sh - Check foundational node status/statistics (requires 'bc' installed)

# Get timestamps
lastbingo=`grep -i bingo latest/*.log |tail -n 1 |cut -f 2 -d 'r' |cut -c 16-34 | tr T \ `
time1=$(date -d "$lastbingo" +%s)
rawtime=`date +%s`
time2=$(($rawtime - 60))
created=`grep -i -m 1 allocated latest/*.log |cut -f 7-9 -d ":" |cut -f 1 -d "." |cut -c 2- | tr T \ `
latest=`grep -i bingo latest/*.log |tail -n 1 |cut -f 2 -d 'T' |cut -c 1-14`
bingos=`grep -i bingo latest/*.log |tail -n 2 |cut -f 2 -d 'T' |cut -c 7-14`

echo
echo -e '    Date and time     :  \c' && date

# Check validation status
if [[ $time1 -ge $time2 ]]
  then
    echo -e '    Node status       :  Validating OK!\c'
  else
    echo -e '    Node status       :  NOT OK, PANIC!!!\c'
fi

# Fetch current and previous bingo time
curtime=$time1
if [[ -f "prevtime" ]]
  then
    prevtime=$(< prevtime)
  else
    prevtime=0
fi

passed=$(($curtime - $prevtime))

# Find and save the first time stamp of the log
if [[ "$created" != "" ]]
  then
    starttime=$(date -d "$created" +%s)
  else
    starttime=0
fi

# Check balances (wait for delay in update)
echo
echo "    Loading balances... (May take up to 7 seconds)"
timediff=$(($rawtime - $time1))
if [[ $timediff -lt 7 ]]
  then
    sleep $((7 - timediff))
    balances=`./wallet.sh balances`
  else
    balances=`./wallet.sh balances`
fi

# Sum balances
bal0=`echo "$balances" |grep -i "shard 0" |cut -f 2 -d ":" |cut -f1 -d "," | rev | cut -c 5- | rev`
bal1=`echo "$balances" |grep -i "shard 1" |cut -f 2 -d ":" |cut -f1 -d "," | rev | cut -c 5- | rev`
bal2=`echo "$balances" |grep -i "shard 2" |cut -f 2 -d ":" |cut -f1 -d "," | rev | cut -c 5- | rev`
bal3=`echo "$balances" |grep -i "shard 3" |cut -f 2 -d ":" |cut -f1 -d "," | rev | cut -c 5- | rev`
curbal=`echo "$bal0 + $bal1 + $bal2 + $bal3" | bc -l`

# Store previous balance and time if 24 hours has passed, or if node has been restarted/prevtime doesn't exist
if [ $passed -ge 86400 ] || [ $starttime -ge $prevtime ]
  then
    echo $curbal > prevbal
    echo $curtime > prevtime
    prevtime=$curtime
    passed=0
fi

# Calculate rewards per 24 h
prevbal=$(< prevbal)
reward=`echo "$curbal - $prevbal" | bc -l`
if [[ $passed != 0 ]]
  then
    daily=`echo "scale=5; $reward * 86400 / $passed" | bc -l`
  else
    daily=0
fi

# Show results
echo "$balances" |grep -i "address"
if [[ $bal0 != "  0." ]]
  then
    echo "$balances" |grep -i "shard 0" |cut -f 1 -d ","
fi
if [[ $bal1 != "  0." ]]
  then
    echo "$balances" |grep -i "shard 1" |cut -f 1 -d ","
fi
if [[ $bal2 != "  0." ]]
  then
    echo "$balances" |grep -i "shard 2" |cut -f 1 -d ","
fi
if [[ $bal3 != "  0." ]]
  then
    echo "$balances" |grep -i "shard 3" |cut -f 1 -d ","
fi
echo
echo -e '    Saved Bal. Diff.  :  \c'
echo $reward
echo -e '    Time Since Saved  :  \c'
((sec=passed%60, passed/=60, min=passed%60, hrs=passed/60))
timestamp=$(printf "%02d:%02d:%02d" $hrs $min $sec)

echo $timestamp
echo -e '    Est. Rewards (24h):  \c'
echo $daily

# Find and show the time of the latest bingo
echo -e '    Latest Bingo      :  \c' && echo $latest

# Fetch the time/seconds of the last two bingos
bingo1=`echo $bingos |cut -c 1-8`
bingo2=`echo $bingos |cut -c 10-17`

# Delete leading zero
bingo11=`echo $bingo1 |cut -c 1`
if [[ $bingo11 -eq "0" ]]
  then
    bingo1=`echo $bingo1 |cut -c 2`
fi

# Delete leading zero
bingo21=`echo $bingo2 |cut -c 1`
if [[ $bingo21 -eq "0" ]]
  then
    bingo2=`echo $bingo2 |cut -c 2`
fi

# Ensure bingo2 is greater than bingo 1
if (( $(echo "$bingo2 <= $bingo1" |bc -l) ))
  then
    bingo2=`echo "$bingo2 + 60" | bc -l`
fi

# Calculate and show the time difference
echo -e '    Last Bingo Intrvl : ' $(echo $bingo2-$bingo1 | bc -l) 'seconds'
echo
