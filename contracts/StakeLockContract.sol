pragma solidity >=0.4.22;

contract StakeLockContract {
    /**
     * @dev Error messages for require statements
     */
    string internal constant ALREADY_LOCKED = 'Tokens already locked';
    string internal constant NO_TOKEN_UNLOCKABLE = 'No tokens unlockable';
    string internal constant AMOUNT_ZERO = 'Amount can not be 0';

    uint256 internal constant LOCK_PERIOD_IN_EPOCHS = 3;  // Final locking period TBD.

    uint256 internal numBlocksPerEpoch = 5;  // This value is for testing only

    /**
     * @dev locked token structure
     */
    struct lockedToken {
        uint256 _amount;           // The amount of token locked
        uint256 _blockNum;         // The number of the block when the token was locked
        uint256 _epochNum;         // The epoch when the token was locked
        uint256 _lockPeriodCount;  // The number of locking period the token will be locked.
        uint256 _index;            // The index in the addressList
    }

    /**
     * @dev Holds number & validity of tokens locked for a given reason for
     *      a specified address
     */
    mapping(address => lockedToken) private locked;
    mapping(address => uint) private indices;
    address[] private addressList;

    event Locked(address indexed _of, uint _amount, uint256 _epoch);
    event Unlocked(address indexed account, uint index);

    /**
     * @dev Locks a specified amount of tokens against an address
     *      starting at the specific epoch
     */
    function lock()
        public
        payable
        returns (bool)
    {
        // If tokens are already locked, then functions extendLock or
        // increaseLockAmount should be used to make any changes
        require(balanceOf(msg.sender) == 0, ALREADY_LOCKED);
        require(msg.value != 0, AMOUNT_ZERO);

        // By default, the tokens can only be locked for one locking period.
        locked[msg.sender] = lockedToken(msg.value, block.number, currentEpoch(), 1, addressList.push(msg.sender) - 1);

        emit Locked(msg.sender, msg.value, currentEpoch());
        return true;
    }

    /**
     * @dev Unlocks the unlockable tokens of a specified address
     */
    function unlock()
        public
        returns (uint256 unlockableTokens)
    {
        unlockableTokens = getUnlockableTokens(msg.sender);  // For now the unlockableTokens is all the tokens for a address
        require(unlockableTokens != 0, NO_TOKEN_UNLOCKABLE);

        uint indexToRemove = locked[msg.sender]._index;
        delete locked[msg.sender];
        addressList[indexToRemove] = addressList[addressList.length - 1];
        locked[addressList[addressList.length - 1]]._index = indexToRemove;
        addressList.length--;

        msg.sender.transfer(unlockableTokens);
    }

    /**
     * @dev Gets the unlockable tokens of a specified address
     * @param _of The address to query the unlockable token
     */
    function getUnlockableTokens(address _of)
        public
        view
        returns (uint256 unlockableTokens)
    {
        uint256 currentEpoch = currentEpoch();
        if (locked[_of]._epochNum + locked[_of]._lockPeriodCount * LOCK_PERIOD_IN_EPOCHS < currentEpoch) {
            unlockableTokens = locked[_of]._amount;
        }
    }

    /**
     * @dev Gets the token balance of a specified address
     * @param _of The address to query the token balance
     */
    function balanceOf(address _of)
        public
        view
        returns (uint256 balance)
    {
        balance = locked[_of]._amount;
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

    /**
     * @dev Lists all the locked address data.
     */
    function listLockedAddresses()
        public
        view
        returns (address[] memory lockedAddresses, uint256[] memory blockNums, uint256[] memory lockPeriodCounts)
    {
        lockedAddresses = addressList;
        blockNums = new uint256[](addressList.length);
        lockPeriodCounts = new uint256[](addressList.length);
        for (uint i = 0; i < lockedAddresses.length; i++) {
            blockNums[i] = locked[lockedAddresses[i]]._blockNum;
            lockPeriodCounts[i] = locked[lockedAddresses[i]]._lockPeriodCount;
        }
    }
}
