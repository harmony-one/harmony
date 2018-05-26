LEADER_PORT=3333
SLAVE_START_PORT=3000
SLAVE_END_PORT=3009
for port in `seq $SLAVE_START_PORT $SLAVE_END_PORT`;
do
    go run ./slave.go -port $port &
done
go run ./leader.go -port $LEADER_PORT &