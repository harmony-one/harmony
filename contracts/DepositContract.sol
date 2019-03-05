pragma solidity >=0.4.22;

contract DepositContract {
    mapping(address => uint) private stakes;

    function deposit() public payable returns(uint) {
        stakes[msg.sender] += msg.value;
        return stakes[msg.sender];
    }

    function withdraw(uint withdrawAmount) public returns (uint remainingStake) {
        if (withdrawAmount <= stakes[msg.sender]){
            stakes[msg.sender] -= withdrawAmount;
            msg.sender.transfer(withdrawAmount);
            return stakes[msg.sender];  //Overdraft protection, no money is withdrawn!
        }
    }

    function balance() public view returns (uint) {
        return stakes[msg.sender];
    }

    function stakesBalance() public view returns (uint) {
        return address(this).balance;
    }
}
