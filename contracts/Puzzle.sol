pragma solidity >=0.4.22; 

contract Puzzle {
    string internal constant INSUFFICIENT_FUND_MESSAGE = "Insufficient Fund";
    string internal constant RESTRICTED_MESSAGE = "Unauthorized Access";
    string internal constant INCORRECT_SESSION = "Player requesting payout for unknown session";
    string internal constant INCORRECT_LEVEL = "Player requesting payout for earlier level";
    uint amount;
    uint sessionID;
    uint progress;
    string seq;
    uint constant thresholdLevel = 4;
    mapping(address => uint) playerLevel;
    mapping(address => uint) playerSession;
    mapping(address => uint) playerStake;
    address public manager;  // The adress of the owner of this contract
    address payable[] public players;   // The players of current session

    constructor() public {
        manager = msg.sender;
    }
    
    /**
     * @dev random number generator
     *     
     */
    function random() private returns (uint) {
        sessionID = uint(keccak256(abi.encodePacked(now, msg.sender, this)));
        return sessionID;   
    }
    
    /**
     * @dev The player enters into the current game session by
     *      paying at least 2 token.
     */
    function play() public payable returns (uint) {
        require(msg.value >= 2 ether, INSUFFICIENT_FUND_MESSAGE);
        sessionID = random();
        playerLevel[msg.sender] = 0;
        playerSession[msg.sender] = sessionID;
        playerStake[msg.sender] = msg.value;
        players.push(msg.sender);
        return sessionID;
    }

    /**
     * @dev returns the current best level the player has crossed   
     */
    function setLevel(address player, uint level) public{
         playerLevel[player] = level;
    }
    
    /**
     * @dev pay the player if they have crossed their last best level.
     */
    function payout(address payable player, uint level, uint session, string memory /*sequence*/) public payable restricted {
            require(playerSession[player] == session, INCORRECT_SESSION );
            require(level > playerLevel[player], INCORRECT_LEVEL);
            progress = level - playerLevel[player]; //if a later transaction for a higher level comes earlier.
            setLevel(player,level);
            amount = progress*(playerStake[player]/thresholdLevel);
            player.transfer(amount);
    }

    modifier restricted() {
        require(msg.sender == manager, RESTRICTED_MESSAGE);
        _;
    }
    
    /**
     * @dev Resets the current session of one player
     */
    function resetPlayer(address player) public restricted {
            delete playerLevel[player];
            delete playerSession[player];
            delete playerStake[player]; 
    }
    
    /**
     * @dev Resets the current session by deleting all the players
     */
    function reset() public restricted {
        uint arrayLength = players.length;
        for(uint i=0; i< arrayLength; i++) {
            address player = players[i];
            resetPlayer(player);
        }
        players.length = 0;
    }
    
    /**
     * @dev Returns a list of all players in the current session.
     */
    function getPlayers() public view returns (address payable[] memory) {
        return players;
    }
}
