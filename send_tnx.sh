for port in `seq 0 9`;
# the hex is basically    0,             0032,               4,    "9001 9002 9003 9004 9005 9006 9007 9008 9009 9010"
#        explanation:  type,   size(50 bytes), [START_CONSENSUS,    payload]
#                                               START_CONSENSUS +   payload = 50 bytes
do
    echo -e '\x00\x00\x00\x00\x32\x04\x39\x30\x30\x31\x20\x39\x30\x30\x32\x20\x39\x30\x30\x33\x20\x39\x30\x30\x34\x20\x39\x30\x30\x35\x20\x39\x30\x30\x36\x20\x39\x30\x30\x37\x20\x39\x30\x30\x38\x20\x39\x30\x30\x39\x20\x39\x30\x31\x30' | nc 127.0.0.1 9000
done