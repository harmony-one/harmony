pragma solidity >=0.4.22; 
contract Hexie {
    string internal constant INSUFFICIENT_FUND_MESSAGE = "Insufficient Fund";
    string internal constant RESTRICTED_MESSAGE = "Unauthorized Access";

    mapping(address => uint) bestlevel;
    address public manager;  // The adress of the owner of this contract
    address payable[] public players;  // The players of current session

    constructor() public {
        manager = msg.sender;
    }

    /**
     * @dev The player enters into the current game session by
     *      paying at least 2 token.
     */
    function play() public payable {
        require(msg.value > 2 ether, INSUFFICIENT_FUND_MESSAGE);
        players.push(msg.sender);
    }

    /**
     * @dev returns the current best level the player has crossed   
     */
    function getlevel() public uint8 {
        return bestlevel[msg.sender]
    }

    /**
     * @dev returns the current best level the player has crossed   
     */
    function setlevel(address player,uint level) public uint {
       bestlevel[player] = level
    }

    /**
     * @dev pay the player if they have crossed their last best level. Set tjeir
     */
    function payout(address player, uint level) public payable restricted {
        if ( level > bestlevel[player]) {
            amount = level - bestlevel[player] 
            setlevel(player,level)
            player.transfer(amount);
        }
    }

    modifier restricted() {
        require(msg.sender == manager, RESTRICTED_MESSAGE);
        _;
    }

    /**
     * @dev Returns a list of all players in the current session.
     */
    function getPlayers() public view returns (address payable[] memory) {
        return players;
    }
}
