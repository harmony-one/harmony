#!/usr/bin/env bash

# stat.sh - Check foundational node status/statistics (requires 'bc' installed)

# Get timestamps
created=$(grep -i -m 1 allocated latest/*.log |cut -f 7-9 -d ":" |\
          cut -f 1 -d "." |cut -c 2- |tr T \ )
bingolist=$(grep -i bingo latest/*.log)
lastbingo=$(echo "$bingolist" |tail -n 1 |cut -f 2 -d 'r' |\
            cut -c 16-34 |tr T \ )
latest=$(echo "$bingolist" |tail -n 1 |cut -f 2 -d 'T' |cut -c 1-14)
bingos=$(echo "$bingolist" |tail -n 2 |cut -f 2 -d 'T' |cut -c 7-14)
time1=$(date -d "$lastbingo" +%s)
rawtime=$(date +%s)
time2=$(($rawtime - 60))

echo
echo -e '    Date and time     :  \c' && date

# Check validation status
if [[ $time1 -ge $time2 ]]
    then
        echo "    Node status       :  Validating OK!"
    else
        echo "    Node status       :  NOT OK, PANIC!!!"
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
echo "    Loading balances... (May take up to 7 seconds)"
timediff=$(($rawtime - $time1))
if [[ $timediff -lt 7 ]]
    then
        sleep $((7 - timediff))
fi
balances=$(./wallet.sh balances)

# Sum balances
numshards=$(echo "$balances" |grep -c "Shard")
declare -a bal
for (( i=0; i < $numshards; i++ ))
do
    bal[$i]=$(echo "$balances" |grep "d $i" |cut -f 2 -d ":" |\
              cut -f 1 -d "," |rev |cut -c 5- |rev)
done
curbal=$(echo "${bal[@]/%/+}0" | bc)

# Store previous balance and time if 24 hours has passed,
# or if node has been restarted/prevtime doesn't exist
if [ $passed -ge 86400 ] || [ $starttime -ge $prevtime ]
    then
        echo "$curbal" > prevbal
        echo "$curtime" > prevtime
        prevtime=$curtime
        passed=0
fi

# Calculate rewards per 24 h
prevbal=$(< prevbal)
reward=$(echo "$curbal - $prevbal" | bc -l)
if [[ $passed != 0 ]]
    then
        daily=$(echo "scale=5; $reward * 86400 / $passed" | bc -l)
    else
        daily=0
fi

# Calculate time difference
((sec=passed%60, passed/=60, min=passed%60, hrs=passed/60))
timestamp=$(printf "%02d:%02d:%02d" $hrs $min $sec)

# Show address and balances
echo "$balances" |grep -i "address"
for (( b=0; i < ${#bal[@]}; b++ ))
do
    if [[ ${bal[$b]} != "  0." ]]
        then
            echo "$balances" |grep "Shard $b" |cut -f 1 -d ","
    fi
done
echo

# Show differences and results
echo "    Saved Bal. Diff.  :  $reward"
echo "    Time Since Saved  :  $timestamp"
echo "    Est. Rewards (24h):  $daily"

# Find and show the time of the latest bingo
echo "    Latest Bingo      :  $latest"

# Fetch the time/seconds of the last two bingos
bingo1=$(echo $bingos |cut -c 1-8)
bingo2=$(echo $bingos |cut -c 10-17)

# Ensure bingo2 is greater than bingo 1
if (( $(echo "$bingo2 <= $bingo1" |bc -l) ))
    then
        bingo2=$(echo "$bingo2 + 60" | bc -l)
fi

# Calculate and show the time difference
difference=$(echo $bingo2-$bingo1 | bc -l)
echo "    Last Bingo Intrvl :  $difference seconds"
echo