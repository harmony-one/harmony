pragma solidity >=0.4.22;

contract StakeLockContract {
    /**
     * @dev Error messages for require statements
     */
    string internal constant ALREADY_LOCKED = 'Tokens already locked';
    string internal constant NOT_LOCKED = 'No tokens locked';
    string internal constant AMOUNT_ZERO = 'Amount can not be 0';

    uint256 internal constant LOCK_PERIOD_IN_EPOCHS = 3;  // Final locking period TBD.

    uint256 internal numBlocksPerEpoch = 5;  // This value is for testing only

    /**
     * @dev locked token structure
     */
    struct lockedToken {
        uint256 _amount;
        uint256 _blockNum;
        uint256 _epochNum;
        uint256 _continueCount;
    }

    /**
     * @dev Holds number & validity of tokens locked for a given reason for
     *      a specified address
     */
    mapping(address => lockedToken) private locked;
    mapping(address => uint) private indices;
    address[] private accountList;

    event Locked(address indexed _of, uint _amount, uint256 _epoch);
    event Unlocked(address indexed account, uint index);

    /**
     * @dev Returns total tokens held by an address (locked + transferable)
     * @param _of The address to query the total balance of
     */
    function lockedBalanceOf(address _of)
        public
        view
        returns (uint256 amount)
    {
        amount = locked[_of]._amount;
    }

    /**
     * @dev Locks a specified amount of tokens against an address
     *      starting at the specific epoch
     * @param _epoch The epoch where the lock happens
     */
    function lock(uint256 _epoch)
        public
        payable
        returns (bool)
    {
        // If tokens are already locked, then functions extendLock or
        // increaseLockAmount should be used to make any changes
        require(lockedBalanceOf(msg.sender) == 0, ALREADY_LOCKED);
        require(msg.value != 0, AMOUNT_ZERO);

        locked[msg.sender] = lockedToken(msg.value, block.number, currentEpoch(), 1);

        emit Locked(msg.sender, msg.value, _epoch);
        return true;
    }

    /**
     * @dev Unlocks the unlockable tokens of a specified address
     * @param _requestor Address of user, claiming back unlockable tokens
     */
    function unlock(address payable _requestor)
        public
        returns (uint256 unlockableTokens)
    {
        unlockableTokens = getUnlockableTokens(_requestor);

        if (unlockableTokens != 0) {
            _requestor.transfer(unlockableTokens);
        }
    }

    /**
     * @dev Gets the unlockable tokens of a specified address
     * @param _of The address to query the the unlockable token count of
     */
    function getUnlockableTokens(address _of)
        public
        view
        returns (uint256 unlockableTokens)
    {
        uint256 currentEpoch = currentEpoch();
        if (locked[_of]._epochNum + locked[_of]._continueCount * LOCK_PERIOD_IN_EPOCHS < currentEpoch) {
            unlockableTokens = locked[_of]._amount;
        }
    }

    /**
     * @dev Gets the epoch number of the specified block number.
     */
    function currentEpoch()
        public
        view
        returns (uint256)
    {
        return block.number / numBlocksPerEpoch;
    }
}
