pragma solidity >=0.4.22;

contract DepositContract {
    mapping(address => uint) private Stakes;
    mapping(address => uint) private Indices;
    address[] private accountList;

    event LogNewAccount(address indexed account, uint index, uint stake);
    event LogUpdateStake(address indexed account, uint index,  uint stake);
    event LogDeleteAccount(address indexed account, uint index);
  
    function isAccount(address account) public view returns(bool isIndeed) {
        if(accountList.length == 0) return false;
        return (accountList[Indices[account]] == account);
    }

    function deposit() public payable returns(uint index) {
        if(isAccount(msg.sender)) {
            Stakes[msg.sender] += msg.value;
            emit LogUpdateStake(msg.sender, Indices[msg.sender], Stakes[msg.sender]);
            return Indices[msg.sender];
        } else {
            Stakes[msg.sender] = msg.value;
            Indices[msg.sender] = accountList.push(msg.sender) - 1;
            emit LogNewAccount(msg.sender, Indices[msg.sender], Stakes[msg.sender]);
            return accountList.length-1;
        }
    }

    function withdraw(uint withdrawAmount) public returns (uint remainingStake) {
        if (withdrawAmount <= Stakes[msg.sender]){
            Stakes[msg.sender] -= withdrawAmount;
            msg.sender.transfer(withdrawAmount);
            if (Stakes[msg.sender] == 0) { //If all stake has been withdrawn, remove the Account.
                deleteAccount(msg.sender);
            }
            return Stakes[msg.sender];
        } else {
            return Stakes[msg.sender];  //Overdraft protection, no money is withdrawn!
        }        
    }
  
    function deleteAccount(address account) public returns(uint index) {
      	//Move the last index to deleted index.
        uint indexToDelete = Indices[account];
        address lastIndexAddress = accountList[accountList.length-1];
        accountList[indexToDelete] = lastIndexAddress;
        Indices[lastIndexAddress] = indexToDelete; 
        accountList.length--;
        emit LogDeleteAccount(account,indexToDelete);
        emit LogUpdateStake(lastIndexAddress, indexToDelete, Stakes[lastIndexAddress]);
        return indexToDelete;
    }

    function getAccountCount() public view returns(uint count) {
        return accountList.length;
    }

    function getStakedAccountList() public view returns(address[] memory stakedAccountList) {
        return accountList;
    }
}