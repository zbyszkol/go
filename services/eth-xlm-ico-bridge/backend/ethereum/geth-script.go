package ethereum

const gethListenerScript = `
function streamTransactionsToAccount(myaccount, currentBlock) {
  while (true) {
    var block = eth.getBlock(currentBlock, true);
    if (block == null) {
      console.log("Block "+currentBlock+" does not exist! Waiting...")
      admin.sleepBlocks(1);
      continue;
    }

    console.log("Block "+currentBlock+" exists!");

    if (block.transactions != null) {
      block.transactions.forEach(function(tx) {
        console.log(JSON.stringify(tx));
      });
    }

    currentBlock++;
  }
}

streamTransactionsToAccount(eth.accounts[0], eth.blockNumber);
`
