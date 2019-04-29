pragma solidity >=0.4.22;

contract Lottery {
    string internal constant INSUFFICIENT_FUND_MESSAGE = "Insufficient Fund";
    string internal constant RESTRICTED_MESSAGE = "Unauthorized Access";

    address public manager;  // The adress of the owner of this contract
    address payable[] public players;  // The players of current session

    constructor() public {
        manager = msg.sender;
    }

    /**
     * @dev The player enters into the current lottery session by
     *      paying at least 0.1 token.
     */
    function enter() public payable {
        require(msg.value > .1 ether, INSUFFICIENT_FUND_MESSAGE);

        players.push(msg.sender);
    }

    /**
     * @dev Random number used for picking the winner.
     */
    function random() private view returns (uint) {
        return uint(keccak256(abi.encodePacked(now, msg.sender, this)));
    }

    /**
     * @dev Randomly select one of the players in current session as winner
     *      and send all the token in smart contract balance to it.
     */
    function pickWinner() public payable restricted {
        uint index = random() % players.length;
        players[index].transfer(address(this).balance);
        players = new address payable[](0);
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
