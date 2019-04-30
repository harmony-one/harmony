pragma solidity >=0.4.22; 
contract Puzzle {
    string internal constant INSUFFICIENT_FUND_MESSAGE = "Insufficient Fund";
    string internal constant RESTRICTED_MESSAGE = "Unauthorized Access";
    address public manager;  // The adress of the owner of this contract
    mapping(address => uint8) public level_map;

    constructor() public payable {
        manager = msg.sender;
    }

    /**
     * @dev The player enters into the current game session by
     *      paying at least 2 token.
     */
    function play() public payable {
        require(msg.value >= 2 ether, INSUFFICIENT_FUND_MESSAGE);
    }

    /**
     * @dev pay the player if they have crossed their last best level.
     */
    function payout(address payable player, uint8 new_level) public payable restricted {
        uint8 cur_level = level_map[player];
        if (new_level >= cur_level) {
            player.transfer(2 * (new_level - cur_level + 1) * 1 ether);
            level_map[player] = new_level;
        } else {
            delete level_map[player];
        }
    }

    modifier restricted() {
        require(msg.sender == manager, RESTRICTED_MESSAGE);
        _;
    }
}
