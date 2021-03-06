// Copyright (c) 2020-2021 The bitcoinpay developers

package blkmgr

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/btceasypay/bitcoinpay/common/hash"
	"github.com/btceasypay/bitcoinpay/common/marshal"
	"github.com/btceasypay/bitcoinpay/core/blockchain"
	"github.com/btceasypay/bitcoinpay/core/blockdag"
	"github.com/btceasypay/bitcoinpay/core/json"
	"github.com/btceasypay/bitcoinpay/core/types"
	"github.com/btceasypay/bitcoinpay/engine/txscript"
	"github.com/btceasypay/bitcoinpay/rpc"
	"strconv"
)

func (b *BlockManager) GetChain() *blockchain.BlockChain {
	return b.chain
}
func (b *BlockManager) API() rpc.API {
	return rpc.API{
		NameSpace: rpc.DefaultServiceNameSpace,
		Service:   NewPublicBlockAPI(b),
		Public:    true,
	}
}

type PublicBlockAPI struct {
	bm *BlockManager
}

func NewPublicBlockAPI(bm *BlockManager) *PublicBlockAPI {
	return &PublicBlockAPI{bm}
}

//TODO, refactor BlkMgr API
func (api *PublicBlockAPI) GetBlockhash(order uint) (string, error) {
	blockHash, err := api.bm.chain.BlockHashByOrder(uint64(order))
	if err != nil {
		return "", err
	}
	return blockHash.String(), nil
}

// Return the hash range of block from 'start' to 'end'(exclude self)
// if 'end' is equal to zero, 'start' is the number that from the last block to the Gen
// if 'start' is greater than or equal to 'end', it will just return the hash of 'start'
func (api *PublicBlockAPI) GetBlockhashByRange(start uint, end uint) ([]string, error) {
	totalOrder := api.bm.chain.BlockDAG().GetBlockTotal()
	if start >= totalOrder {
		return nil, fmt.Errorf("startOrder(%d) is greater than or equal to the totalOrder(%d)", start, totalOrder)
	}
	result := []string{}
	if start >= end && end != 0 {
		block, err := api.bm.chain.BlockByOrder(uint64(start))
		if err != nil {
			return nil, err
		}
		result = append(result, block.Hash().String())
	} else if end == 0 {
		for i := totalOrder - 1; i >= 0; i-- {
			if uint(len(result)) >= start {
				break
			}
			block, err := api.bm.chain.BlockByOrder(uint64(i))
			if err != nil {
				return nil, err
			}
			result = append(result, block.Hash().String())
		}
	} else {
		for i := start; i < totalOrder; i++ {
			if i >= end {
				break
			}
			block, err := api.bm.chain.BlockByOrder(uint64(i))
			if err != nil {
				return nil, err
			}
			result = append(result, block.Hash().String())
		}
	}
	return result, nil
}

func (api *PublicBlockAPI) GetBlockByOrder(order uint64, verbose *bool, inclTx *bool, fullTx *bool) (interface{}, error) {
	if uint(order) > api.bm.chain.BestSnapshot().GraphState.GetMainOrder() {
		return nil, fmt.Errorf("Order is too big")
	}
	blockHash, err := api.bm.chain.BlockHashByOrder(order)
	if err != nil {
		return nil, err
	}
	vb := false
	if verbose != nil {
		vb = *verbose
	}
	iTx := true
	if inclTx != nil {
		iTx = *inclTx
	}
	fTx := true
	if fullTx != nil {
		fTx = *fullTx
	}
	return api.GetBlock(*blockHash, &vb, &iTx, &fTx)
}

func (api *PublicBlockAPI) GetBlock(h hash.Hash, verbose *bool, inclTx *bool, fullTx *bool) (interface{}, error) {

	vb := false
	if verbose != nil {
		vb = *verbose
	}
	iTx := true
	if inclTx != nil {
		iTx = *inclTx
	}
	fTx := true
	if fullTx != nil {
		fTx = *fullTx
	}

	// Load the raw block bytes from the database.
	// Note :
	// FetchBlockByHash differs from BlockByHash in that this one also returns blocks
	// that are not part of the main chain (if they are known).
	blk, err := api.bm.chain.FetchBlockByHash(&h)
	if err != nil {
		return nil, err
	}
	node := api.bm.chain.BlockIndex().LookupNode(&h)
	if node == nil {
		return nil, fmt.Errorf("no node")
	}
	// Update the source block order
	blk.SetOrder(node.GetOrder())
	blk.SetHeight(node.GetHeight())
	// When the verbose flag isn't set, simply return the
	// network-serialized block as a hex-encoded string.
	if !vb {
		blkBytes, err := blk.Bytes()
		if err != nil {
			return nil, rpc.RpcInternalError(err.Error(),
				"Could not serialize block")
		}
		return hex.EncodeToString(blkBytes), nil
	}
	confirmations := int64(api.bm.chain.BlockDAG().GetConfirmations(node.GetID()))
	ib := api.bm.chain.BlockDAG().GetBlock(&h)
	cs := ib.GetChildren()
	children := []*hash.Hash{}
	if cs != nil && !cs.IsEmpty() {
		for _, v := range cs.GetMap() {
			children = append(children, v.(blockdag.IBlock).GetHash())
		}
	}
	api.bm.chain.CalculateDAGDuplicateTxs(blk)
	coinbaseAmout := blk.Transactions()[0].Tx.TxOut[0].Amount + uint64(api.bm.chain.CalculateFees(blk))

	//TODO, refactor marshal api
	fields, err := marshal.MarshalJsonBlock(blk, iTx, fTx, api.bm.params, confirmations, children,
		!api.bm.chain.BlockIndex().NodeStatus(node).KnownInvalid(), node.IsOrdered(), coinbaseAmout, 0)
	if err != nil {
		return nil, err
	}
	return fields, nil
}

func (api *PublicBlockAPI) GetBlockV2(h hash.Hash, verbose *bool, inclTx *bool, fullTx *bool) (interface{}, error) {

	vb := false
	if verbose != nil {
		vb = *verbose
	}
	iTx := true
	if inclTx != nil {
		iTx = *inclTx
	}
	fTx := true
	if fullTx != nil {
		fTx = *fullTx
	}

	// Load the raw block bytes from the database.
	// Note :
	// FetchBlockByHash differs from BlockByHash in that this one also returns blocks
	// that are not part of the main chain (if they are known).
	blk, err := api.bm.chain.FetchBlockByHash(&h)
	if err != nil {
		return nil, err
	}
	node := api.bm.chain.BlockIndex().LookupNode(&h)
	if node == nil {
		return nil, fmt.Errorf("no node")
	}
	// Update the source block order
	blk.SetOrder(node.GetOrder())
	blk.SetHeight(node.GetHeight())
	// When the verbose flag isn't set, simply return the
	// network-serialized block as a hex-encoded string.
	if !vb {
		blkBytes, err := blk.Bytes()
		if err != nil {
			return nil, rpc.RpcInternalError(err.Error(),
				"Could not serialize block")
		}
		return hex.EncodeToString(blkBytes), nil
	}
	confirmations := int64(api.bm.chain.BlockDAG().GetConfirmations(node.GetID()))
	ib := api.bm.chain.BlockDAG().GetBlock(&h)
	cs := ib.GetChildren()
	children := []*hash.Hash{}
	if cs != nil && !cs.IsEmpty() {
		for _, v := range cs.GetMap() {
			children = append(children, v.(blockdag.IBlock).GetHash())
		}
	}
	api.bm.chain.CalculateDAGDuplicateTxs(blk)
	coinbaseAmout := blk.Transactions()[0].Tx.TxOut[0].Amount
	coinbaseFee := uint64(api.bm.chain.CalculateFees(blk))

	//TODO, refactor marshal api
	fields, err := marshal.MarshalJsonBlock(blk, iTx, fTx, api.bm.params, confirmations, children,
		!api.bm.chain.BlockIndex().NodeStatus(node).KnownInvalid(), node.IsOrdered(), coinbaseAmout, coinbaseFee)
	if err != nil {
		return nil, err
	}
	return fields, nil

}

func (api *PublicBlockAPI) GetBestBlockHash() (interface{}, error) {
	best := api.bm.chain.BestSnapshot()
	return best.Hash.String(), nil
}

// The total ordered Block count
func (api *PublicBlockAPI) GetBlockCount() (interface{}, error) {
	best := api.bm.chain.BestSnapshot()
	return best.GraphState.GetMainOrder() + 1, nil
}

// The total Block count, included possible blocks have not ordered by BlockDAG consensus yet at the moments.
func (api *PublicBlockAPI) GetBlockTotal() (interface{}, error) {
	best := api.bm.chain.BestSnapshot()
	return best.GraphState.GetTotal(), nil
}

// GetBlockHeader implements the getblockheader command.
func (api *PublicBlockAPI) GetBlockHeader(hash hash.Hash, verbose bool) (interface{}, error) {

	// Fetch the block node
	node := api.bm.chain.BlockIndex().LookupNode(&hash)
	if node == nil {
		return nil, rpc.RpcInternalError(fmt.Errorf("no block").Error(), fmt.Sprintf("Block not found: %v", hash))
	}
	// Fetch the header from chain.
	blockHeader, err := api.bm.chain.HeaderByHash(&hash)
	if err != nil {
		return nil, rpc.RpcInternalError(err.Error(), fmt.Sprintf("Block not found: %v", hash))
	}

	// When the verbose flag isn't set, simply return the serialized block
	// header as a hex-encoded string.
	if !verbose {
		var headerBuf bytes.Buffer
		err := blockHeader.Serialize(&headerBuf)
		if err != nil {
			context := "Failed to serialize block header"
			return nil, rpc.RpcInternalError(err.Error(), context)
		}
		return hex.EncodeToString(headerBuf.Bytes()), nil
	}
	// Get next block hash unless there are none.
	confirmations := int64(api.bm.chain.BlockDAG().GetConfirmations(node.GetID()))
	layer := api.bm.chain.BlockDAG().GetLayer(node.GetID())
	blockHeaderReply := json.GetBlockHeaderVerboseResult{
		Hash:          hash.String(),
		Confirmations: confirmations,
		Version:       int32(blockHeader.Version),
		ParentRoot:    blockHeader.ParentRoot.String(),
		TxRoot:        blockHeader.TxRoot.String(),
		StateRoot:     blockHeader.StateRoot.String(),
		Difficulty:    blockHeader.Difficulty,
		Layer:         uint32(layer),
		Time:          blockHeader.Timestamp.Unix(),
		PowResult:     blockHeader.Pow.GetPowResult(),
	}

	return blockHeaderReply, nil

}

// Query whether a given block is on the main chain.
// Note that some DAG protocols may not support this feature.
func (api *PublicBlockAPI) IsOnMainChain(h hash.Hash) (interface{}, error) {
	node := api.bm.chain.BlockIndex().LookupNode(&h)
	if node == nil {
		return nil, rpc.RpcInternalError(fmt.Errorf("no block").Error(), fmt.Sprintf("Block not found: %v", h))
	}
	isOn := api.bm.chain.BlockDAG().IsOnMainChain(node.GetID())

	return strconv.FormatBool(isOn), nil
}

// Return the current height of DAG main chain
func (api *PublicBlockAPI) GetMainChainHeight() (interface{}, error) {
	return strconv.FormatUint(uint64(api.bm.GetChain().BlockDAG().GetMainChainTip().GetHeight()), 10), nil
}

// Return the weight of block
func (api *PublicBlockAPI) GetBlockWeight(h hash.Hash) (interface{}, error) {
	block, err := api.bm.chain.FetchBlockByHash(&h)
	if err != nil {
		return nil, rpc.RpcInternalError(fmt.Errorf("no block").Error(), fmt.Sprintf("Block not found: %v", h))
	}
	return strconv.FormatInt(int64(types.GetBlockWeight(block.Block())), 10), nil
}

// Return the total number of orphan blocks, orphan block are the blocks have not been included into the DAG at this moment.
func (api *PublicBlockAPI) GetOrphansTotal() (interface{}, error) {
	return api.bm.GetChain().GetOrphansTotal(), nil
}

// Obsoleted GetBlockByID Method, since the confused naming, replaced by GetBlockByNum method
func (api *PublicBlockAPI) GetBlockByID(id uint64, verbose *bool, inclTx *bool, fullTx *bool) (interface{}, error) {
	return api.GetBlockByNum(id, verbose, inclTx, fullTx)
}

// GetBlockByNum works like GetBlockByOrder, the different is the GetBlockByNum is return the order result from
// the current node's DAG directly instead of according to the consensus of BlockDAG algorithm.
func (api *PublicBlockAPI) GetBlockByNum(num uint64, verbose *bool, inclTx *bool, fullTx *bool) (interface{}, error) {
	blockHash := api.bm.GetChain().BlockDAG().GetBlockHash(uint(num))
	if blockHash == nil {
		return nil, rpc.RpcInternalError(fmt.Errorf("no block").Error(), fmt.Sprintf("Block not found: %v", num))
	}
	vb := false
	if verbose != nil {
		vb = *verbose
	}
	iTx := true
	if inclTx != nil {
		iTx = *inclTx
	}
	fTx := true
	if fullTx != nil {
		fTx = *fullTx
	}
	return api.GetBlock(*blockHash, &vb, &iTx, &fTx)
}

// IsBlue:0:not blue;  1：blue  2：Cannot confirm
func (api *PublicBlockAPI) IsBlue(h hash.Hash) (interface{}, error) {
	ib := api.bm.chain.BlockDAG().GetBlock(&h)
	if ib == nil {
		return 2, rpc.RpcInternalError(fmt.Errorf("no block").Error(), fmt.Sprintf("Block not found: %s", h.String()))
	}
	confirmations := api.bm.chain.BlockDAG().GetConfirmations(ib.GetID())
	if confirmations == 0 {
		return 2, nil
	}
	if api.bm.chain.BlockDAG().IsBlue(ib.GetID()) {
		return 1, nil
	}
	return 0, nil
}

// Return IsCurrent
func (api *PublicBlockAPI) IsCurrent() (interface{}, error) {
	return api.bm.IsCurrent(), nil
}

// Return a list hash of the tip blocks of the DAG at this moment.
func (api *PublicBlockAPI) Tips() (interface{}, error) {
	tipsList, err := api.bm.TipGeneration()
	if err != nil {
		return nil, err
	}
	tips := []string{}
	for _, v := range tipsList {
		tips = append(tips, v.String())
	}
	return tips, nil
}

// GetCoinbase
func (api *PublicBlockAPI) GetCoinbase(h hash.Hash, verbose *bool) (interface{}, error) {
	vb := false
	if verbose != nil {
		vb = *verbose
	}
	blk, err := api.bm.chain.FetchBlockByHash(&h)
	if err != nil {
		return nil, err
	}
	signDatas, err := txscript.ExtractCoinbaseData(blk.Block().Transactions[0].TxIn[0].SignScript)
	if err != nil {
		return nil, err
	}
	result := []string{}
	for k, v := range signDatas {
		if k < 2 && !vb {
			continue
		}
		result = append(result, hex.EncodeToString(v))
	}
	return result, nil
}

// GetCoinbase
func (api *PublicBlockAPI) GetFees(h hash.Hash) (interface{}, error) {
	return api.bm.chain.GetFees(&h), nil
}
