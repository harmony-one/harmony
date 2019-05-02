pragma solidity >=0.4.22; 

contract Puzzle {
    string internal constant INSUFFICIENT_FUND_MESSAGE = "Insufficient Fund";
    string internal constant RESTRICTED_MESSAGE = "Unauthorized Access";
    string internal constant NOT_IN_GAME = "Player is not in an active game";
    string internal constant INCORRECT_LEVEL = "Player requesting payout for earlier level";

    uint constant thresholdLevel = 4;
    mapping(address => uint) playerLevel;
    mapping(address => uint) playerStake;
    mapping(address => bool) playerInGame;  // Whether the player is in a active game or not. Payout won't work if not in game.
    mapping(address => bool) playerSet;

    address public manager;  // The adress of the owner of this contract
    address payable[] public players;   // all players

    constructor() public payable {
        manager = msg.sender;
    }

    /**
     * @dev The player enters into a new game by
     *      paying at least 20 token.
     */
    function play() public payable {
        require(msg.value >= 20 ether, INSUFFICIENT_FUND_MESSAGE);

        if (playerSet[msg.sender] == false) {
            // New player, never staked before
            playerSet[msg.sender] = true;
            players.push(msg.sender);
        }
        playerLevel[msg.sender] = 0;
        playerStake[msg.sender] = msg.value;
        playerInGame[msg.sender] = true;
    }

    /**
     * @dev set the current best level the player has crossed
     */
    function setLevel(address player, uint level) public {
         playerLevel[player] = level;
    }

    /**
     * @dev pay the player if they have crossed their last best level.
     */
    function payout(address payable player, uint level, string memory /*sequence*/) public {
            require(playerInGame[player] == true, NOT_IN_GAME);

            uint progress = level - playerLevel[player]; //if a later transaction for a higher level comes earlier.
            setLevel(player,level);
            uint amount = progress*(playerStake[player]/thresholdLevel);
            player.transfer(amount);
    }

    /**
     * @dev set the player's game state to inactive.
     */
    function endGame(address player) public {
            delete playerInGame[player];
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
            delete playerInGame[player];
            delete playerStake[player];
            delete playerSet[player];
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
