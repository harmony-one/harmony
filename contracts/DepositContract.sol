pragma solidity ^0.4.22;

contract DepositContract {

  mapping(address => uint) private Stakes;
  mapping(address => uint) private Indices;
  address[] private nodeAddressList;

  event LogNewNode(address indexed nodeAddress, uint index, uint stake);
  event LogUpdateStake(address indexed nodeAddress, uint index,  uint stake);
  event LogDeleteNode(address indexed nodeAddress, uint index);
  
  function isNodeAddress(address nodeAddress) public constant returns(bool isIndeed) 
  {
    if(nodeAddressList.length == 0) return false;
    return (nodeAddressList[Indices[nodeAddress]] == nodeAddress);
  }

function deposit() public payable returns(uint index)
  {
    if(isNodeAddress(msg.sender)) {
    	Stakes[msg.sender] += msg.value;
    	emit LogUpdateStake(msg.sender, Indices[msg.sender], Stakes[msg.sender]);
   		return Indices[msg.sender];
    } else {
    	Stakes[msg.sender]   = msg.value;
	    Indices[msg.sender] = nodeAddressList.push(msg.sender) - 1;
    	emit LogNewNode(msg.sender, Indices[msg.sender], Stakes[msg.sender]);
    	return nodeAddressList.length-1;
    }
  }

  function withdraw(uint withdrawAmount) public returns (uint remainingStake) 
  {
        if (withdrawAmount  <= Stakes[msg.sender]) {
        	 Stakes[msg.sender] -= withdrawAmount;
             msg.sender.transfer(withdrawAmount);
             if (Stakes[msg.sender] == 0) { //If all stake has been withdrawn, remove the node.
             	deleteNode(msg.sender);
             }
            return Stakes[msg.sender];
        } else {
        	return Stakes[msg.sender];  //Overdraft protection, no money is withdrawn!
        }
        
    }
  
  function deleteNode(address nodeAddress) public returns(uint index)
  {
  	//Move the last index to deleted index.
    uint indexToDelete = Indices[nodeAddress];
    address lastIndexAddress = nodeAddressList[nodeAddressList.length-1];
    nodeAddressList[indexToDelete] = lastIndexAddress;
    Indices[lastIndexAddress] = indexToDelete; 
    nodeAddressList.length--;
    emit LogDeleteNode(nodeAddress,indexToDelete);
    emit LogUpdateStake(lastIndexAddress, indexToDelete, Stakes[lastIndexAddress]);
    return indexToDelete;
  }


  function getNodeCount() public constant returns(uint count)
  {
    return nodeAddressList.length;
  }

  function getStakedNodeList() public constant returns(address[] stakedNodeList)
  {

    return nodeAddressList;
  }



}