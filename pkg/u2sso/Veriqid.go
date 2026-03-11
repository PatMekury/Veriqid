// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package u2sso

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// VeriqidMetaData contains all meta data concerning the Veriqid contract.
var VeriqidMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousAdmin\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newAdmin\",\"type\":\"address\"}],\"name\":\"AdminTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"enumVeriqid.AgeBracket\",\"name\":\"ageBracket\",\"type\":\"uint8\"}],\"name\":\"IDRegistered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"revokedBy\",\"type\":\"address\"}],\"name\":\"IDRevoked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"VerifierAuthorized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"VerifierRemoved\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"admin\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"authorizedVerifiers\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"idList\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"id\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"id33\",\"type\":\"uint256\"},{\"internalType\":\"bool\",\"name\":\"active\",\"type\":\"bool\"},{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"enumVeriqid.AgeBracket\",\"name\":\"ageBracket\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[],\"name\":\"nextIndex\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_verifier\",\"type\":\"address\"}],\"name\":\"authorizeVerifier\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_verifier\",\"type\":\"address\"}],\"name\":\"removeVerifier\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_newAdmin\",\"type\":\"address\"}],\"name\":\"transferAdmin\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_id\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_id33\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"_ageBracket\",\"type\":\"uint8\"}],\"name\":\"addID\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_index\",\"type\":\"uint256\"}],\"name\":\"revokeID\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_index\",\"type\":\"uint256\"}],\"name\":\"getIDs\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_index\",\"type\":\"uint256\"}],\"name\":\"getState\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[],\"name\":\"getIDSize\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_id\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_id33\",\"type\":\"uint256\"}],\"name\":\"getIDIndex\",\"outputs\":[{\"internalType\":\"int256\",\"name\":\"\",\"type\":\"int256\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_index\",\"type\":\"uint256\"}],\"name\":\"getAgeBracket\",\"outputs\":[{\"internalType\":\"enumVeriqid.AgeBracket\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_index\",\"type\":\"uint256\"}],\"name\":\"getOwner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_start\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_count\",\"type\":\"uint256\"}],\"name\":\"getBatchIDs\",\"outputs\":[{\"internalType\":\"uint256[]\",\"name\":\"ids\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256[]\",\"name\":\"id33s\",\"type\":\"uint256[]\"},{\"internalType\":\"bool[]\",\"name\":\"actives\",\"type\":\"bool[]\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[],\"name\":\"getActiveIDCount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_addr\",\"type\":\"address\"}],\"name\":\"isVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true}]",
}

// VeriqidABI is the input ABI used to generate the binding from.
// Deprecated: Use VeriqidMetaData.ABI instead.
var VeriqidABI = VeriqidMetaData.ABI

// Veriqid is an auto generated Go binding around an Ethereum contract.
type Veriqid struct {
	VeriqidCaller     // Read-only binding to the contract
	VeriqidTransactor // Write-only binding to the contract
	VeriqidFilterer   // Log filterer for contract events
}

// VeriqidCaller is an auto generated read-only Go binding around an Ethereum contract.
type VeriqidCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// VeriqidTransactor is an auto generated write-only Go binding around an Ethereum contract.
type VeriqidTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// VeriqidFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type VeriqidFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// VeriqidSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type VeriqidSession struct {
	Contract     *Veriqid          // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// VeriqidCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type VeriqidCallerSession struct {
	Contract *VeriqidCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts  // Call options to use throughout this session
}

// VeriqidTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type VeriqidTransactorSession struct {
	Contract     *VeriqidTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// VeriqidRaw is an auto generated low-level Go binding around an Ethereum contract.
type VeriqidRaw struct {
	Contract *Veriqid // Generic contract binding to access the raw methods on
}

// VeriqidCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type VeriqidCallerRaw struct {
	Contract *VeriqidCaller // Generic read-only contract binding to access the raw methods on
}

// VeriqidTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type VeriqidTransactorRaw struct {
	Contract *VeriqidTransactor // Generic write-only contract binding to access the raw methods on
}

// NewVeriqid creates a new instance of Veriqid, bound to a specific deployed contract.
func NewVeriqid(address common.Address, backend bind.ContractBackend) (*Veriqid, error) {
	contract, err := bindVeriqid(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Veriqid{VeriqidCaller: VeriqidCaller{contract: contract}, VeriqidTransactor: VeriqidTransactor{contract: contract}, VeriqidFilterer: VeriqidFilterer{contract: contract}}, nil
}

// NewVeriqidCaller creates a new read-only instance of Veriqid, bound to a specific deployed contract.
func NewVeriqidCaller(address common.Address, caller bind.ContractCaller) (*VeriqidCaller, error) {
	contract, err := bindVeriqid(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &VeriqidCaller{contract: contract}, nil
}

// NewVeriqidTransactor creates a new write-only instance of Veriqid, bound to a specific deployed contract.
func NewVeriqidTransactor(address common.Address, transactor bind.ContractTransactor) (*VeriqidTransactor, error) {
	contract, err := bindVeriqid(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &VeriqidTransactor{contract: contract}, nil
}

// NewVeriqidFilterer creates a new log filterer instance of Veriqid, bound to a specific deployed contract.
func NewVeriqidFilterer(address common.Address, filterer bind.ContractFilterer) (*VeriqidFilterer, error) {
	contract, err := bindVeriqid(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &VeriqidFilterer{contract: contract}, nil
}

// bindVeriqid binds a generic wrapper to an already deployed contract.
func bindVeriqid(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := VeriqidMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Veriqid *VeriqidRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Veriqid.Contract.VeriqidCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Veriqid *VeriqidRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Veriqid.Contract.VeriqidTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Veriqid *VeriqidRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Veriqid.Contract.VeriqidTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Veriqid *VeriqidCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Veriqid.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Veriqid *VeriqidTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Veriqid.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Veriqid *VeriqidTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Veriqid.Contract.contract.Transact(opts, method, params...)
}

// Admin is a free data retrieval call binding the contract method 0xf851a440.
//
// Solidity: function admin() view returns(address)
func (_Veriqid *VeriqidCaller) Admin(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Veriqid.contract.Call(opts, &out, "admin")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Admin is a free data retrieval call binding the contract method 0xf851a440.
//
// Solidity: function admin() view returns(address)
func (_Veriqid *VeriqidSession) Admin() (common.Address, error) {
	return _Veriqid.Contract.Admin(&_Veriqid.CallOpts)
}

// Admin is a free data retrieval call binding the contract method 0xf851a440.
//
// Solidity: function admin() view returns(address)
func (_Veriqid *VeriqidCallerSession) Admin() (common.Address, error) {
	return _Veriqid.Contract.Admin(&_Veriqid.CallOpts)
}

// AuthorizedVerifiers is a free data retrieval call binding the contract method 0x9a891716.
//
// Solidity: function authorizedVerifiers(address ) view returns(bool)
func (_Veriqid *VeriqidCaller) AuthorizedVerifiers(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var out []interface{}
	err := _Veriqid.contract.Call(opts, &out, "authorizedVerifiers", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// AuthorizedVerifiers is a free data retrieval call binding the contract method 0x9a891716.
//
// Solidity: function authorizedVerifiers(address ) view returns(bool)
func (_Veriqid *VeriqidSession) AuthorizedVerifiers(arg0 common.Address) (bool, error) {
	return _Veriqid.Contract.AuthorizedVerifiers(&_Veriqid.CallOpts, arg0)
}

// AuthorizedVerifiers is a free data retrieval call binding the contract method 0x9a891716.
//
// Solidity: function authorizedVerifiers(address ) view returns(bool)
func (_Veriqid *VeriqidCallerSession) AuthorizedVerifiers(arg0 common.Address) (bool, error) {
	return _Veriqid.Contract.AuthorizedVerifiers(&_Veriqid.CallOpts, arg0)
}

// GetActiveIDCount is a free data retrieval call binding the contract method 0xae6cdec4.
//
// Solidity: function getActiveIDCount() view returns(uint256)
func (_Veriqid *VeriqidCaller) GetActiveIDCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Veriqid.contract.Call(opts, &out, "getActiveIDCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetActiveIDCount is a free data retrieval call binding the contract method 0xae6cdec4.
//
// Solidity: function getActiveIDCount() view returns(uint256)
func (_Veriqid *VeriqidSession) GetActiveIDCount() (*big.Int, error) {
	return _Veriqid.Contract.GetActiveIDCount(&_Veriqid.CallOpts)
}

// GetActiveIDCount is a free data retrieval call binding the contract method 0xae6cdec4.
//
// Solidity: function getActiveIDCount() view returns(uint256)
func (_Veriqid *VeriqidCallerSession) GetActiveIDCount() (*big.Int, error) {
	return _Veriqid.Contract.GetActiveIDCount(&_Veriqid.CallOpts)
}

// GetAgeBracket is a free data retrieval call binding the contract method 0x66bd785c.
//
// Solidity: function getAgeBracket(uint256 _index) view returns(uint8)
func (_Veriqid *VeriqidCaller) GetAgeBracket(opts *bind.CallOpts, _index *big.Int) (uint8, error) {
	var out []interface{}
	err := _Veriqid.contract.Call(opts, &out, "getAgeBracket", _index)

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// GetAgeBracket is a free data retrieval call binding the contract method 0x66bd785c.
//
// Solidity: function getAgeBracket(uint256 _index) view returns(uint8)
func (_Veriqid *VeriqidSession) GetAgeBracket(_index *big.Int) (uint8, error) {
	return _Veriqid.Contract.GetAgeBracket(&_Veriqid.CallOpts, _index)
}

// GetAgeBracket is a free data retrieval call binding the contract method 0x66bd785c.
//
// Solidity: function getAgeBracket(uint256 _index) view returns(uint8)
func (_Veriqid *VeriqidCallerSession) GetAgeBracket(_index *big.Int) (uint8, error) {
	return _Veriqid.Contract.GetAgeBracket(&_Veriqid.CallOpts, _index)
}

// GetBatchIDs is a free data retrieval call binding the contract method 0x2485dced.
//
// Solidity: function getBatchIDs(uint256 _start, uint256 _count) view returns(uint256[] ids, uint256[] id33s, bool[] actives)
func (_Veriqid *VeriqidCaller) GetBatchIDs(opts *bind.CallOpts, _start *big.Int, _count *big.Int) (struct {
	Ids     []*big.Int
	Id33s   []*big.Int
	Actives []bool
}, error) {
	var out []interface{}
	err := _Veriqid.contract.Call(opts, &out, "getBatchIDs", _start, _count)

	outstruct := new(struct {
		Ids     []*big.Int
		Id33s   []*big.Int
		Actives []bool
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Ids = *abi.ConvertType(out[0], new([]*big.Int)).(*[]*big.Int)
	outstruct.Id33s = *abi.ConvertType(out[1], new([]*big.Int)).(*[]*big.Int)
	outstruct.Actives = *abi.ConvertType(out[2], new([]bool)).(*[]bool)

	return *outstruct, err

}

// GetBatchIDs is a free data retrieval call binding the contract method 0x2485dced.
//
// Solidity: function getBatchIDs(uint256 _start, uint256 _count) view returns(uint256[] ids, uint256[] id33s, bool[] actives)
func (_Veriqid *VeriqidSession) GetBatchIDs(_start *big.Int, _count *big.Int) (struct {
	Ids     []*big.Int
	Id33s   []*big.Int
	Actives []bool
}, error) {
	return _Veriqid.Contract.GetBatchIDs(&_Veriqid.CallOpts, _start, _count)
}

// GetBatchIDs is a free data retrieval call binding the contract method 0x2485dced.
//
// Solidity: function getBatchIDs(uint256 _start, uint256 _count) view returns(uint256[] ids, uint256[] id33s, bool[] actives)
func (_Veriqid *VeriqidCallerSession) GetBatchIDs(_start *big.Int, _count *big.Int) (struct {
	Ids     []*big.Int
	Id33s   []*big.Int
	Actives []bool
}, error) {
	return _Veriqid.Contract.GetBatchIDs(&_Veriqid.CallOpts, _start, _count)
}

// GetIDIndex is a free data retrieval call binding the contract method 0x9a80d1dd.
//
// Solidity: function getIDIndex(uint256 _id, uint256 _id33) view returns(int256)
func (_Veriqid *VeriqidCaller) GetIDIndex(opts *bind.CallOpts, _id *big.Int, _id33 *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _Veriqid.contract.Call(opts, &out, "getIDIndex", _id, _id33)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetIDIndex is a free data retrieval call binding the contract method 0x9a80d1dd.
//
// Solidity: function getIDIndex(uint256 _id, uint256 _id33) view returns(int256)
func (_Veriqid *VeriqidSession) GetIDIndex(_id *big.Int, _id33 *big.Int) (*big.Int, error) {
	return _Veriqid.Contract.GetIDIndex(&_Veriqid.CallOpts, _id, _id33)
}

// GetIDIndex is a free data retrieval call binding the contract method 0x9a80d1dd.
//
// Solidity: function getIDIndex(uint256 _id, uint256 _id33) view returns(int256)
func (_Veriqid *VeriqidCallerSession) GetIDIndex(_id *big.Int, _id33 *big.Int) (*big.Int, error) {
	return _Veriqid.Contract.GetIDIndex(&_Veriqid.CallOpts, _id, _id33)
}

// GetIDSize is a free data retrieval call binding the contract method 0x2d3d1104.
//
// Solidity: function getIDSize() view returns(uint256)
func (_Veriqid *VeriqidCaller) GetIDSize(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Veriqid.contract.Call(opts, &out, "getIDSize")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetIDSize is a free data retrieval call binding the contract method 0x2d3d1104.
//
// Solidity: function getIDSize() view returns(uint256)
func (_Veriqid *VeriqidSession) GetIDSize() (*big.Int, error) {
	return _Veriqid.Contract.GetIDSize(&_Veriqid.CallOpts)
}

// GetIDSize is a free data retrieval call binding the contract method 0x2d3d1104.
//
// Solidity: function getIDSize() view returns(uint256)
func (_Veriqid *VeriqidCallerSession) GetIDSize() (*big.Int, error) {
	return _Veriqid.Contract.GetIDSize(&_Veriqid.CallOpts)
}

// GetIDs is a free data retrieval call binding the contract method 0x6f1acd98.
//
// Solidity: function getIDs(uint256 _index) view returns(uint256, uint256)
func (_Veriqid *VeriqidCaller) GetIDs(opts *bind.CallOpts, _index *big.Int) (*big.Int, *big.Int, error) {
	var out []interface{}
	err := _Veriqid.contract.Call(opts, &out, "getIDs", _index)

	if err != nil {
		return *new(*big.Int), *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	out1 := *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return out0, out1, err

}

// GetIDs is a free data retrieval call binding the contract method 0x6f1acd98.
//
// Solidity: function getIDs(uint256 _index) view returns(uint256, uint256)
func (_Veriqid *VeriqidSession) GetIDs(_index *big.Int) (*big.Int, *big.Int, error) {
	return _Veriqid.Contract.GetIDs(&_Veriqid.CallOpts, _index)
}

// GetIDs is a free data retrieval call binding the contract method 0x6f1acd98.
//
// Solidity: function getIDs(uint256 _index) view returns(uint256, uint256)
func (_Veriqid *VeriqidCallerSession) GetIDs(_index *big.Int) (*big.Int, *big.Int, error) {
	return _Veriqid.Contract.GetIDs(&_Veriqid.CallOpts, _index)
}

// GetOwner is a free data retrieval call binding the contract method 0xc41a360a.
//
// Solidity: function getOwner(uint256 _index) view returns(address)
func (_Veriqid *VeriqidCaller) GetOwner(opts *bind.CallOpts, _index *big.Int) (common.Address, error) {
	var out []interface{}
	err := _Veriqid.contract.Call(opts, &out, "getOwner", _index)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetOwner is a free data retrieval call binding the contract method 0xc41a360a.
//
// Solidity: function getOwner(uint256 _index) view returns(address)
func (_Veriqid *VeriqidSession) GetOwner(_index *big.Int) (common.Address, error) {
	return _Veriqid.Contract.GetOwner(&_Veriqid.CallOpts, _index)
}

// GetOwner is a free data retrieval call binding the contract method 0xc41a360a.
//
// Solidity: function getOwner(uint256 _index) view returns(address)
func (_Veriqid *VeriqidCallerSession) GetOwner(_index *big.Int) (common.Address, error) {
	return _Veriqid.Contract.GetOwner(&_Veriqid.CallOpts, _index)
}

// GetState is a free data retrieval call binding the contract method 0x44c9af28.
//
// Solidity: function getState(uint256 _index) view returns(bool)
func (_Veriqid *VeriqidCaller) GetState(opts *bind.CallOpts, _index *big.Int) (bool, error) {
	var out []interface{}
	err := _Veriqid.contract.Call(opts, &out, "getState", _index)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// GetState is a free data retrieval call binding the contract method 0x44c9af28.
//
// Solidity: function getState(uint256 _index) view returns(bool)
func (_Veriqid *VeriqidSession) GetState(_index *big.Int) (bool, error) {
	return _Veriqid.Contract.GetState(&_Veriqid.CallOpts, _index)
}

// GetState is a free data retrieval call binding the contract method 0x44c9af28.
//
// Solidity: function getState(uint256 _index) view returns(bool)
func (_Veriqid *VeriqidCallerSession) GetState(_index *big.Int) (bool, error) {
	return _Veriqid.Contract.GetState(&_Veriqid.CallOpts, _index)
}

// IdList is a free data retrieval call binding the contract method 0x6313531f.
//
// Solidity: function idList(uint256 ) view returns(uint256 id, uint256 id33, bool active, address owner, uint8 ageBracket)
func (_Veriqid *VeriqidCaller) IdList(opts *bind.CallOpts, arg0 *big.Int) (struct {
	Id         *big.Int
	Id33       *big.Int
	Active     bool
	Owner      common.Address
	AgeBracket uint8
}, error) {
	var out []interface{}
	err := _Veriqid.contract.Call(opts, &out, "idList", arg0)

	outstruct := new(struct {
		Id         *big.Int
		Id33       *big.Int
		Active     bool
		Owner      common.Address
		AgeBracket uint8
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Id = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.Id33 = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.Active = *abi.ConvertType(out[2], new(bool)).(*bool)
	outstruct.Owner = *abi.ConvertType(out[3], new(common.Address)).(*common.Address)
	outstruct.AgeBracket = *abi.ConvertType(out[4], new(uint8)).(*uint8)

	return *outstruct, err

}

// IdList is a free data retrieval call binding the contract method 0x6313531f.
//
// Solidity: function idList(uint256 ) view returns(uint256 id, uint256 id33, bool active, address owner, uint8 ageBracket)
func (_Veriqid *VeriqidSession) IdList(arg0 *big.Int) (struct {
	Id         *big.Int
	Id33       *big.Int
	Active     bool
	Owner      common.Address
	AgeBracket uint8
}, error) {
	return _Veriqid.Contract.IdList(&_Veriqid.CallOpts, arg0)
}

// IdList is a free data retrieval call binding the contract method 0x6313531f.
//
// Solidity: function idList(uint256 ) view returns(uint256 id, uint256 id33, bool active, address owner, uint8 ageBracket)
func (_Veriqid *VeriqidCallerSession) IdList(arg0 *big.Int) (struct {
	Id         *big.Int
	Id33       *big.Int
	Active     bool
	Owner      common.Address
	AgeBracket uint8
}, error) {
	return _Veriqid.Contract.IdList(&_Veriqid.CallOpts, arg0)
}

// IsVerifier is a free data retrieval call binding the contract method 0x33105218.
//
// Solidity: function isVerifier(address _addr) view returns(bool)
func (_Veriqid *VeriqidCaller) IsVerifier(opts *bind.CallOpts, _addr common.Address) (bool, error) {
	var out []interface{}
	err := _Veriqid.contract.Call(opts, &out, "isVerifier", _addr)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsVerifier is a free data retrieval call binding the contract method 0x33105218.
//
// Solidity: function isVerifier(address _addr) view returns(bool)
func (_Veriqid *VeriqidSession) IsVerifier(_addr common.Address) (bool, error) {
	return _Veriqid.Contract.IsVerifier(&_Veriqid.CallOpts, _addr)
}

// IsVerifier is a free data retrieval call binding the contract method 0x33105218.
//
// Solidity: function isVerifier(address _addr) view returns(bool)
func (_Veriqid *VeriqidCallerSession) IsVerifier(_addr common.Address) (bool, error) {
	return _Veriqid.Contract.IsVerifier(&_Veriqid.CallOpts, _addr)
}

// NextIndex is a free data retrieval call binding the contract method 0xfc7e9c6f.
//
// Solidity: function nextIndex() view returns(uint256)
func (_Veriqid *VeriqidCaller) NextIndex(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Veriqid.contract.Call(opts, &out, "nextIndex")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NextIndex is a free data retrieval call binding the contract method 0xfc7e9c6f.
//
// Solidity: function nextIndex() view returns(uint256)
func (_Veriqid *VeriqidSession) NextIndex() (*big.Int, error) {
	return _Veriqid.Contract.NextIndex(&_Veriqid.CallOpts)
}

// NextIndex is a free data retrieval call binding the contract method 0xfc7e9c6f.
//
// Solidity: function nextIndex() view returns(uint256)
func (_Veriqid *VeriqidCallerSession) NextIndex() (*big.Int, error) {
	return _Veriqid.Contract.NextIndex(&_Veriqid.CallOpts)
}

// AddID is a paid mutator transaction binding the contract method 0x883daa91.
//
// Solidity: function addID(uint256 _id, uint256 _id33, uint8 _ageBracket) returns(uint256)
func (_Veriqid *VeriqidTransactor) AddID(opts *bind.TransactOpts, _id *big.Int, _id33 *big.Int, _ageBracket uint8) (*types.Transaction, error) {
	return _Veriqid.contract.Transact(opts, "addID", _id, _id33, _ageBracket)
}

// AddID is a paid mutator transaction binding the contract method 0x883daa91.
//
// Solidity: function addID(uint256 _id, uint256 _id33, uint8 _ageBracket) returns(uint256)
func (_Veriqid *VeriqidSession) AddID(_id *big.Int, _id33 *big.Int, _ageBracket uint8) (*types.Transaction, error) {
	return _Veriqid.Contract.AddID(&_Veriqid.TransactOpts, _id, _id33, _ageBracket)
}

// AddID is a paid mutator transaction binding the contract method 0x883daa91.
//
// Solidity: function addID(uint256 _id, uint256 _id33, uint8 _ageBracket) returns(uint256)
func (_Veriqid *VeriqidTransactorSession) AddID(_id *big.Int, _id33 *big.Int, _ageBracket uint8) (*types.Transaction, error) {
	return _Veriqid.Contract.AddID(&_Veriqid.TransactOpts, _id, _id33, _ageBracket)
}

// AuthorizeVerifier is a paid mutator transaction binding the contract method 0x20ad8cab.
//
// Solidity: function authorizeVerifier(address _verifier) returns()
func (_Veriqid *VeriqidTransactor) AuthorizeVerifier(opts *bind.TransactOpts, _verifier common.Address) (*types.Transaction, error) {
	return _Veriqid.contract.Transact(opts, "authorizeVerifier", _verifier)
}

// AuthorizeVerifier is a paid mutator transaction binding the contract method 0x20ad8cab.
//
// Solidity: function authorizeVerifier(address _verifier) returns()
func (_Veriqid *VeriqidSession) AuthorizeVerifier(_verifier common.Address) (*types.Transaction, error) {
	return _Veriqid.Contract.AuthorizeVerifier(&_Veriqid.TransactOpts, _verifier)
}

// AuthorizeVerifier is a paid mutator transaction binding the contract method 0x20ad8cab.
//
// Solidity: function authorizeVerifier(address _verifier) returns()
func (_Veriqid *VeriqidTransactorSession) AuthorizeVerifier(_verifier common.Address) (*types.Transaction, error) {
	return _Veriqid.Contract.AuthorizeVerifier(&_Veriqid.TransactOpts, _verifier)
}

// RemoveVerifier is a paid mutator transaction binding the contract method 0xca2dfd0a.
//
// Solidity: function removeVerifier(address _verifier) returns()
func (_Veriqid *VeriqidTransactor) RemoveVerifier(opts *bind.TransactOpts, _verifier common.Address) (*types.Transaction, error) {
	return _Veriqid.contract.Transact(opts, "removeVerifier", _verifier)
}

// RemoveVerifier is a paid mutator transaction binding the contract method 0xca2dfd0a.
//
// Solidity: function removeVerifier(address _verifier) returns()
func (_Veriqid *VeriqidSession) RemoveVerifier(_verifier common.Address) (*types.Transaction, error) {
	return _Veriqid.Contract.RemoveVerifier(&_Veriqid.TransactOpts, _verifier)
}

// RemoveVerifier is a paid mutator transaction binding the contract method 0xca2dfd0a.
//
// Solidity: function removeVerifier(address _verifier) returns()
func (_Veriqid *VeriqidTransactorSession) RemoveVerifier(_verifier common.Address) (*types.Transaction, error) {
	return _Veriqid.Contract.RemoveVerifier(&_Veriqid.TransactOpts, _verifier)
}

// RevokeID is a paid mutator transaction binding the contract method 0xce3375d5.
//
// Solidity: function revokeID(uint256 _index) returns()
func (_Veriqid *VeriqidTransactor) RevokeID(opts *bind.TransactOpts, _index *big.Int) (*types.Transaction, error) {
	return _Veriqid.contract.Transact(opts, "revokeID", _index)
}

// RevokeID is a paid mutator transaction binding the contract method 0xce3375d5.
//
// Solidity: function revokeID(uint256 _index) returns()
func (_Veriqid *VeriqidSession) RevokeID(_index *big.Int) (*types.Transaction, error) {
	return _Veriqid.Contract.RevokeID(&_Veriqid.TransactOpts, _index)
}

// RevokeID is a paid mutator transaction binding the contract method 0xce3375d5.
//
// Solidity: function revokeID(uint256 _index) returns()
func (_Veriqid *VeriqidTransactorSession) RevokeID(_index *big.Int) (*types.Transaction, error) {
	return _Veriqid.Contract.RevokeID(&_Veriqid.TransactOpts, _index)
}

// TransferAdmin is a paid mutator transaction binding the contract method 0x75829def.
//
// Solidity: function transferAdmin(address _newAdmin) returns()
func (_Veriqid *VeriqidTransactor) TransferAdmin(opts *bind.TransactOpts, _newAdmin common.Address) (*types.Transaction, error) {
	return _Veriqid.contract.Transact(opts, "transferAdmin", _newAdmin)
}

// TransferAdmin is a paid mutator transaction binding the contract method 0x75829def.
//
// Solidity: function transferAdmin(address _newAdmin) returns()
func (_Veriqid *VeriqidSession) TransferAdmin(_newAdmin common.Address) (*types.Transaction, error) {
	return _Veriqid.Contract.TransferAdmin(&_Veriqid.TransactOpts, _newAdmin)
}

// TransferAdmin is a paid mutator transaction binding the contract method 0x75829def.
//
// Solidity: function transferAdmin(address _newAdmin) returns()
func (_Veriqid *VeriqidTransactorSession) TransferAdmin(_newAdmin common.Address) (*types.Transaction, error) {
	return _Veriqid.Contract.TransferAdmin(&_Veriqid.TransactOpts, _newAdmin)
}

// VeriqidAdminTransferredIterator is returned from FilterAdminTransferred and is used to iterate over the raw logs and unpacked data for AdminTransferred events raised by the Veriqid contract.
type VeriqidAdminTransferredIterator struct {
	Event *VeriqidAdminTransferred // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *VeriqidAdminTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(VeriqidAdminTransferred)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(VeriqidAdminTransferred)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *VeriqidAdminTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *VeriqidAdminTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// VeriqidAdminTransferred represents a AdminTransferred event raised by the Veriqid contract.
type VeriqidAdminTransferred struct {
	PreviousAdmin common.Address
	NewAdmin      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterAdminTransferred is a free log retrieval operation binding the contract event 0xf8ccb027dfcd135e000e9d45e6cc2d662578a8825d4c45b5e32e0adf67e79ec6.
//
// Solidity: event AdminTransferred(address indexed previousAdmin, address indexed newAdmin)
func (_Veriqid *VeriqidFilterer) FilterAdminTransferred(opts *bind.FilterOpts, previousAdmin []common.Address, newAdmin []common.Address) (*VeriqidAdminTransferredIterator, error) {

	var previousAdminRule []interface{}
	for _, previousAdminItem := range previousAdmin {
		previousAdminRule = append(previousAdminRule, previousAdminItem)
	}
	var newAdminRule []interface{}
	for _, newAdminItem := range newAdmin {
		newAdminRule = append(newAdminRule, newAdminItem)
	}

	logs, sub, err := _Veriqid.contract.FilterLogs(opts, "AdminTransferred", previousAdminRule, newAdminRule)
	if err != nil {
		return nil, err
	}
	return &VeriqidAdminTransferredIterator{contract: _Veriqid.contract, event: "AdminTransferred", logs: logs, sub: sub}, nil
}

// WatchAdminTransferred is a free log subscription operation binding the contract event 0xf8ccb027dfcd135e000e9d45e6cc2d662578a8825d4c45b5e32e0adf67e79ec6.
//
// Solidity: event AdminTransferred(address indexed previousAdmin, address indexed newAdmin)
func (_Veriqid *VeriqidFilterer) WatchAdminTransferred(opts *bind.WatchOpts, sink chan<- *VeriqidAdminTransferred, previousAdmin []common.Address, newAdmin []common.Address) (event.Subscription, error) {

	var previousAdminRule []interface{}
	for _, previousAdminItem := range previousAdmin {
		previousAdminRule = append(previousAdminRule, previousAdminItem)
	}
	var newAdminRule []interface{}
	for _, newAdminItem := range newAdmin {
		newAdminRule = append(newAdminRule, newAdminItem)
	}

	logs, sub, err := _Veriqid.contract.WatchLogs(opts, "AdminTransferred", previousAdminRule, newAdminRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(VeriqidAdminTransferred)
				if err := _Veriqid.contract.UnpackLog(event, "AdminTransferred", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAdminTransferred is a log parse operation binding the contract event 0xf8ccb027dfcd135e000e9d45e6cc2d662578a8825d4c45b5e32e0adf67e79ec6.
//
// Solidity: event AdminTransferred(address indexed previousAdmin, address indexed newAdmin)
func (_Veriqid *VeriqidFilterer) ParseAdminTransferred(log types.Log) (*VeriqidAdminTransferred, error) {
	event := new(VeriqidAdminTransferred)
	if err := _Veriqid.contract.UnpackLog(event, "AdminTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// VeriqidIDRegisteredIterator is returned from FilterIDRegistered and is used to iterate over the raw logs and unpacked data for IDRegistered events raised by the Veriqid contract.
type VeriqidIDRegisteredIterator struct {
	Event *VeriqidIDRegistered // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *VeriqidIDRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(VeriqidIDRegistered)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(VeriqidIDRegistered)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *VeriqidIDRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *VeriqidIDRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// VeriqidIDRegistered represents a IDRegistered event raised by the Veriqid contract.
type VeriqidIDRegistered struct {
	Index      *big.Int
	Owner      common.Address
	AgeBracket uint8
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterIDRegistered is a free log retrieval operation binding the contract event 0xe6b3b4f14cd3c72e38db2861ff296be8260eaf59481d990cb48c7d1031f1e83d.
//
// Solidity: event IDRegistered(uint256 indexed index, address indexed owner, uint8 ageBracket)
func (_Veriqid *VeriqidFilterer) FilterIDRegistered(opts *bind.FilterOpts, index []*big.Int, owner []common.Address) (*VeriqidIDRegisteredIterator, error) {

	var indexRule []interface{}
	for _, indexItem := range index {
		indexRule = append(indexRule, indexItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _Veriqid.contract.FilterLogs(opts, "IDRegistered", indexRule, ownerRule)
	if err != nil {
		return nil, err
	}
	return &VeriqidIDRegisteredIterator{contract: _Veriqid.contract, event: "IDRegistered", logs: logs, sub: sub}, nil
}

// WatchIDRegistered is a free log subscription operation binding the contract event 0xe6b3b4f14cd3c72e38db2861ff296be8260eaf59481d990cb48c7d1031f1e83d.
//
// Solidity: event IDRegistered(uint256 indexed index, address indexed owner, uint8 ageBracket)
func (_Veriqid *VeriqidFilterer) WatchIDRegistered(opts *bind.WatchOpts, sink chan<- *VeriqidIDRegistered, index []*big.Int, owner []common.Address) (event.Subscription, error) {

	var indexRule []interface{}
	for _, indexItem := range index {
		indexRule = append(indexRule, indexItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _Veriqid.contract.WatchLogs(opts, "IDRegistered", indexRule, ownerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(VeriqidIDRegistered)
				if err := _Veriqid.contract.UnpackLog(event, "IDRegistered", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseIDRegistered is a log parse operation binding the contract event 0xe6b3b4f14cd3c72e38db2861ff296be8260eaf59481d990cb48c7d1031f1e83d.
//
// Solidity: event IDRegistered(uint256 indexed index, address indexed owner, uint8 ageBracket)
func (_Veriqid *VeriqidFilterer) ParseIDRegistered(log types.Log) (*VeriqidIDRegistered, error) {
	event := new(VeriqidIDRegistered)
	if err := _Veriqid.contract.UnpackLog(event, "IDRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// VeriqidIDRevokedIterator is returned from FilterIDRevoked and is used to iterate over the raw logs and unpacked data for IDRevoked events raised by the Veriqid contract.
type VeriqidIDRevokedIterator struct {
	Event *VeriqidIDRevoked // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *VeriqidIDRevokedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(VeriqidIDRevoked)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(VeriqidIDRevoked)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *VeriqidIDRevokedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *VeriqidIDRevokedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// VeriqidIDRevoked represents a IDRevoked event raised by the Veriqid contract.
type VeriqidIDRevoked struct {
	Index     *big.Int
	RevokedBy common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterIDRevoked is a free log retrieval operation binding the contract event 0xc7b11576686e161ba629cc37aa62844f66f0c875f9547d51b4b9dc5d4a5446ad.
//
// Solidity: event IDRevoked(uint256 indexed index, address indexed revokedBy)
func (_Veriqid *VeriqidFilterer) FilterIDRevoked(opts *bind.FilterOpts, index []*big.Int, revokedBy []common.Address) (*VeriqidIDRevokedIterator, error) {

	var indexRule []interface{}
	for _, indexItem := range index {
		indexRule = append(indexRule, indexItem)
	}
	var revokedByRule []interface{}
	for _, revokedByItem := range revokedBy {
		revokedByRule = append(revokedByRule, revokedByItem)
	}

	logs, sub, err := _Veriqid.contract.FilterLogs(opts, "IDRevoked", indexRule, revokedByRule)
	if err != nil {
		return nil, err
	}
	return &VeriqidIDRevokedIterator{contract: _Veriqid.contract, event: "IDRevoked", logs: logs, sub: sub}, nil
}

// WatchIDRevoked is a free log subscription operation binding the contract event 0xc7b11576686e161ba629cc37aa62844f66f0c875f9547d51b4b9dc5d4a5446ad.
//
// Solidity: event IDRevoked(uint256 indexed index, address indexed revokedBy)
func (_Veriqid *VeriqidFilterer) WatchIDRevoked(opts *bind.WatchOpts, sink chan<- *VeriqidIDRevoked, index []*big.Int, revokedBy []common.Address) (event.Subscription, error) {

	var indexRule []interface{}
	for _, indexItem := range index {
		indexRule = append(indexRule, indexItem)
	}
	var revokedByRule []interface{}
	for _, revokedByItem := range revokedBy {
		revokedByRule = append(revokedByRule, revokedByItem)
	}

	logs, sub, err := _Veriqid.contract.WatchLogs(opts, "IDRevoked", indexRule, revokedByRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(VeriqidIDRevoked)
				if err := _Veriqid.contract.UnpackLog(event, "IDRevoked", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseIDRevoked is a log parse operation binding the contract event 0xc7b11576686e161ba629cc37aa62844f66f0c875f9547d51b4b9dc5d4a5446ad.
//
// Solidity: event IDRevoked(uint256 indexed index, address indexed revokedBy)
func (_Veriqid *VeriqidFilterer) ParseIDRevoked(log types.Log) (*VeriqidIDRevoked, error) {
	event := new(VeriqidIDRevoked)
	if err := _Veriqid.contract.UnpackLog(event, "IDRevoked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// VeriqidVerifierAuthorizedIterator is returned from FilterVerifierAuthorized and is used to iterate over the raw logs and unpacked data for VerifierAuthorized events raised by the Veriqid contract.
type VeriqidVerifierAuthorizedIterator struct {
	Event *VeriqidVerifierAuthorized // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *VeriqidVerifierAuthorizedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(VeriqidVerifierAuthorized)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(VeriqidVerifierAuthorized)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *VeriqidVerifierAuthorizedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *VeriqidVerifierAuthorizedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// VeriqidVerifierAuthorized represents a VerifierAuthorized event raised by the Veriqid contract.
type VeriqidVerifierAuthorized struct {
	Verifier common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterVerifierAuthorized is a free log retrieval operation binding the contract event 0x40f8611cb3470e9d255c7ea6d08e5625d635123c46e7c303e5321b1fb69bb8c9.
//
// Solidity: event VerifierAuthorized(address indexed verifier)
func (_Veriqid *VeriqidFilterer) FilterVerifierAuthorized(opts *bind.FilterOpts, verifier []common.Address) (*VeriqidVerifierAuthorizedIterator, error) {

	var verifierRule []interface{}
	for _, verifierItem := range verifier {
		verifierRule = append(verifierRule, verifierItem)
	}

	logs, sub, err := _Veriqid.contract.FilterLogs(opts, "VerifierAuthorized", verifierRule)
	if err != nil {
		return nil, err
	}
	return &VeriqidVerifierAuthorizedIterator{contract: _Veriqid.contract, event: "VerifierAuthorized", logs: logs, sub: sub}, nil
}

// WatchVerifierAuthorized is a free log subscription operation binding the contract event 0x40f8611cb3470e9d255c7ea6d08e5625d635123c46e7c303e5321b1fb69bb8c9.
//
// Solidity: event VerifierAuthorized(address indexed verifier)
func (_Veriqid *VeriqidFilterer) WatchVerifierAuthorized(opts *bind.WatchOpts, sink chan<- *VeriqidVerifierAuthorized, verifier []common.Address) (event.Subscription, error) {

	var verifierRule []interface{}
	for _, verifierItem := range verifier {
		verifierRule = append(verifierRule, verifierItem)
	}

	logs, sub, err := _Veriqid.contract.WatchLogs(opts, "VerifierAuthorized", verifierRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(VeriqidVerifierAuthorized)
				if err := _Veriqid.contract.UnpackLog(event, "VerifierAuthorized", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseVerifierAuthorized is a log parse operation binding the contract event 0x40f8611cb3470e9d255c7ea6d08e5625d635123c46e7c303e5321b1fb69bb8c9.
//
// Solidity: event VerifierAuthorized(address indexed verifier)
func (_Veriqid *VeriqidFilterer) ParseVerifierAuthorized(log types.Log) (*VeriqidVerifierAuthorized, error) {
	event := new(VeriqidVerifierAuthorized)
	if err := _Veriqid.contract.UnpackLog(event, "VerifierAuthorized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// VeriqidVerifierRemovedIterator is returned from FilterVerifierRemoved and is used to iterate over the raw logs and unpacked data for VerifierRemoved events raised by the Veriqid contract.
type VeriqidVerifierRemovedIterator struct {
	Event *VeriqidVerifierRemoved // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *VeriqidVerifierRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(VeriqidVerifierRemoved)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(VeriqidVerifierRemoved)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *VeriqidVerifierRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *VeriqidVerifierRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// VeriqidVerifierRemoved represents a VerifierRemoved event raised by the Veriqid contract.
type VeriqidVerifierRemoved struct {
	Verifier common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterVerifierRemoved is a free log retrieval operation binding the contract event 0x44a3cd4eb5cc5748f6169df057b1cb2ae4c383e87cd94663c430e095d4cba424.
//
// Solidity: event VerifierRemoved(address indexed verifier)
func (_Veriqid *VeriqidFilterer) FilterVerifierRemoved(opts *bind.FilterOpts, verifier []common.Address) (*VeriqidVerifierRemovedIterator, error) {

	var verifierRule []interface{}
	for _, verifierItem := range verifier {
		verifierRule = append(verifierRule, verifierItem)
	}

	logs, sub, err := _Veriqid.contract.FilterLogs(opts, "VerifierRemoved", verifierRule)
	if err != nil {
		return nil, err
	}
	return &VeriqidVerifierRemovedIterator{contract: _Veriqid.contract, event: "VerifierRemoved", logs: logs, sub: sub}, nil
}

// WatchVerifierRemoved is a free log subscription operation binding the contract event 0x44a3cd4eb5cc5748f6169df057b1cb2ae4c383e87cd94663c430e095d4cba424.
//
// Solidity: event VerifierRemoved(address indexed verifier)
func (_Veriqid *VeriqidFilterer) WatchVerifierRemoved(opts *bind.WatchOpts, sink chan<- *VeriqidVerifierRemoved, verifier []common.Address) (event.Subscription, error) {

	var verifierRule []interface{}
	for _, verifierItem := range verifier {
		verifierRule = append(verifierRule, verifierItem)
	}

	logs, sub, err := _Veriqid.contract.WatchLogs(opts, "VerifierRemoved", verifierRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(VeriqidVerifierRemoved)
				if err := _Veriqid.contract.UnpackLog(event, "VerifierRemoved", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseVerifierRemoved is a log parse operation binding the contract event 0x44a3cd4eb5cc5748f6169df057b1cb2ae4c383e87cd94663c430e095d4cba424.
//
// Solidity: event VerifierRemoved(address indexed verifier)
func (_Veriqid *VeriqidFilterer) ParseVerifierRemoved(log types.Log) (*VeriqidVerifierRemoved, error) {
	event := new(VeriqidVerifierRemoved)
	if err := _Veriqid.contract.UnpackLog(event, "VerifierRemoved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
