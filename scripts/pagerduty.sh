#!/usr/bin/env bash

# Pagerduty constants
LOGFILE="/home/tmp_log/log-20190628.153354/*.log"
LOG=$(tail -n 1000 $logfile)
SHARD_NUM=$(echo "$log" | grep -oE -m 1 "Shard\":.?" | rev | cut -c 1)
IP=$(dig -4 @resolver1.opendns.com ANY myip.opendns.com +short)
KEY="5d11f6173899423689b6fc98691930de"
TRIGGER="\
{\
  \"routing_key\": \"$KEY\",\
  \"event_action\": \"trigger\",\
  \"dedup_key\": \"$IP\",\
  \"payload\": {\
    \"summary\": \"Node NOT validating: Shard-$SHARD_NUM: $IP\",\
    \"source\": \"$IP\",\
    \"severity\": \"critical\"\
  }\
}"
RESOLVE="\
{\
  \"routing_key\": \"$KEY\",\
  \"event_action\": \"resolve\",\
  \"dedup_key\": \"$IP\"\
}"
JSON_HEADER="Content-Type: application/json"
POSTURL="https://events.pagerduty.com/v2/enqueue"

# Bingo check constants
if [[ -f "triggered" ]]
  then
    triggered=$(< triggered)
  else
    triggered=false
fi
lastbingo=$(tail -n 1000 latest/*.log | tac | grep -ai -m 1 bingo)
bingotime=$(echo $lastbingo | cut -f 2 -d 'r' | cut -c 16-34 | tr T \ )
latest=$(date -d "$bingotime" +%s)
curtime=$(date +%s)
delay=60

# Pagerduty alert
if [[ $(($curtime - $latest)) -gt $delay ]]
  then
    cat "$TRIGGER"
    curl -X POST -H "$JSON_HEADER" -d "$TRIGGER" "$POSTURL"
    echo
    triggered=true
  else
    if [[ $triggered = true ]]
      then
        curl -X POST -H "$JSON_HEADER" -d "$RESOLVE" "$POSTURL"
	      echo
       triggered=false
    fi
fi
echo "$triggered" > triggered