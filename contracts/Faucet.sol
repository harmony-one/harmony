pragma solidity >=0.4.22;

contract Faucet {
    mapping(address => bool) processed;
    uint quota = 100 ether;
    address owner;
    constructor() public payable {
        owner = msg.sender;
    }
    function request(address payable requestor) public {
        require(msg.sender == owner);
        require(quota <= address(this).balance);
        require(!processed[requestor]);
        processed[requestor] = true;
        requestor.transfer(quota);
	}
    function money() public view returns(uint) {
        return address(this).balance;
	}
}
