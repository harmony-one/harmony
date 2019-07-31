#!/usr/bin/env bash

### Log variables
LOGFILE="/home/tmp_log/log-20190628.153354/zerolog*.log"
LOG=$(tail -n 1000 $LOGFILE)

### POST constants
IP=$(dig -4 @resolver1.opendns.com ANY myip.opendns.com +short)
SHARD_NUM=$(echo "$LOG" | grep -oE -m 1 "Shard\":.?" | rev | cut -c 1)
TRIGGER="\
{\
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
  \"event_action\": \"resolve\",\
  \"dedup_key\": \"$IP\"\
}"
KEY="${HMY_PAGERDUTY_KEY}"
HEADER="X-Routing-Key: $KEY"
POSTURL="https://events.pagerduty.com/v2/enqueue"

### Validation check variables
validation=$(echo "$LOG" | tac | grep -ai -m 1 "bingo\|hooray")
timestamp=$(echo $validation | cut -f 3 -d 't' | cut -c 4-22 | tr T \ )
latest=$(date -d "$timestamp" +%s)
curtime=$(date +%s)
delay=60

### Pagerduty
if [[ $(($curtime - $latest)) -gt $delay ]]; then
    # Trigger an alert
    trigger=$(curl -s -X POST -H "$HEADER" -d "$TRIGGER" "$POSTURL")
else
    # Resolve an alert
    resolve=$(curl -s -X POST -H "$HEADER" -d "$RESOLVE" "$POSTURL")
fi
