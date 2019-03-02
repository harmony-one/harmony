pragma solidity >=0.4.22;

contract StakeLockContract {
    /**
     * @dev Error messages for require statements
     */
    string internal constant ALREADY_LOCKED = 'Tokens already locked';
    string internal constant NOT_LOCKED = 'No tokens locked';
    string internal constant AMOUNT_ZERO = 'Amount can not be 0';

    /**
     * @dev locked token structure
     */
    struct lockedToken {
        uint256 _amount;
        uint256 _blockNum;
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

        locked[msg.sender] = lockedToken(msg.value, _epoch, 1);

        emit Locked(msg.sender, msg.value, _epoch);
        return true;
    }

    function getAccountCount() public view returns(uint count) {
        return accountList.length;
    }

    function getStakedAccountList() public view returns(address[] memory stakedAccountList) {
        return accountList;
    }
}
