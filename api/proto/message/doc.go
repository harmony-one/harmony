package message

/*

	leader: 								                sent MessageType_ANNOUNCE
	validator: received MessageType_ANNOUNCE 				sent MessageType_PREPARE
	leader:    received MessageType_PREPARE (onPrepare) 	sent MessageType_PREPARED
	validator: received MessageType_PREPARED (onPrepared) 	sent MessageType_COMMIT
	leader:    received MessageType_COMMIT (onCommit) 		sent MessageType_COMMITTED
	validator: received MessageType_COMMITTED (onCommitted) sent MessageType_ANNOUNCE


*/
