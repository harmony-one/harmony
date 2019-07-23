#!/usr/bin/env bash


readarray accounts < "accounts.txt"



start="`date +%s`"

for value in accounts
do
	command="./bin/wallet balances --address $value"
	output=$(eval "$command")
	echo "$output"
done

end="`date +%s`"
totalTime=`expr $end-$start`
echo $totalTime