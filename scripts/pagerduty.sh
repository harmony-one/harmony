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

# Validation check constants
if [[ -f "triggered" ]]
  then
    triggered=$(< triggered)
  else
    triggered=false
fi
validation=$(tail -n 1000 $LOGFILE | tac | grep -ai -m 1 "bingo\|hooray")
timestamp=$(echo $validation | cut -f 3 -d 't' | cut -c 4-22 | tr T \ )
latest=$(date -d "$timestamp" +%s)
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
