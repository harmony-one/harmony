pragma solidity >=0.4.22;

contract Lottery {
    string internal constant ENTER_MESSAGE = "The player needs to stake at least 0.1 ether";
    string internal constant RESTRICTED_MESSAGE = "Only manager can do";

    address public manager;
    address payable[] public players;

    constructor() public {
        manager = msg.sender;
    }

    function enter() public payable {
        require(msg.value > .01 ether, ENTER_MESSAGE);

        players.push(msg.sender);
    }

    function random() private view returns (uint) {
        return uint(keccak256(abi.encodePacked(now, msg.sender, this)));
    }

    function pickWinner() public payable restricted {
        uint index = random() % players.length;
        players[index].transfer(address(this).balance);
        players = new address payable[](0);
    }

    modifier restricted() {
        require(msg.sender == manager, RESTRICTED_MESSAGE);
        _;
    }

    function getPlayers() public view returns (address payable[] memory) {
        return players;
    }
}
