const Veriqid = artifacts.require("Veriqid.sol");

module.exports = function(deployer) {
  deployer.deploy(Veriqid);
};
