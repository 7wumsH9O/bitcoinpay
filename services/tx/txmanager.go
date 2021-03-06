package tx

import (
	"github.com/btceasypay/bitcoinpay/common/hash"
	"github.com/btceasypay/bitcoinpay/config"
	"github.com/btceasypay/bitcoinpay/core/blockchain"
	"github.com/btceasypay/bitcoinpay/core/blockdag"
	"github.com/btceasypay/bitcoinpay/core/types"
	"github.com/btceasypay/bitcoinpay/database"
	"github.com/btceasypay/bitcoinpay/engine/txscript"
	"github.com/btceasypay/bitcoinpay/node/notify"
	"github.com/btceasypay/bitcoinpay/services/blkmgr"
	"github.com/btceasypay/bitcoinpay/services/common"
	"github.com/btceasypay/bitcoinpay/services/index"
	"github.com/btceasypay/bitcoinpay/services/mempool"
	"time"
)

type TxManager struct {
	bm *blkmgr.BlockManager
	// tx index
	txIndex *index.TxIndex

	// addr index
	addrIndex *index.AddrIndex
	// mempool hold tx that need to be mined into blocks and relayed to other peers.
	txMemPool *mempool.TxPool

	// notify
	ntmgr notify.Notify

	// db
	db database.DB

	//invalidTx hash->block hash
	invalidTx map[hash.Hash]*blockdag.HashSet
}

func (tm *TxManager) Start() error {
	log.Info("Starting tx manager")
	return nil
}

func (tm *TxManager) Stop() error {
	log.Info("Stopping tx manager")
	return nil
}

func (tm *TxManager) MemPool() blkmgr.TxPool {
	return tm.txMemPool
}

func NewTxManager(bm *blkmgr.BlockManager, txIndex *index.TxIndex,
	addrIndex *index.AddrIndex, cfg *config.Config, ntmgr notify.Notify,
	sigCache *txscript.SigCache, db database.DB) (*TxManager, error) {
	// mem-pool
	txC := mempool.Config{
		Policy: mempool.Policy{
			MaxTxVersion:         2,
			DisableRelayPriority: cfg.NoRelayPriority,
			AcceptNonStd:         cfg.AcceptNonStd,
			FreeTxRelayLimit:     cfg.FreeTxRelayLimit,
			MaxOrphanTxs:         cfg.MaxOrphanTxs,
			MaxOrphanTxSize:      mempool.DefaultMaxOrphanTxSize,
			MaxSigOpsPerTx:       blockchain.MaxSigOpsPerBlock / 5,
			MinRelayTxFee:        types.Amount(cfg.MinTxFee),
			StandardVerifyFlags: func() (txscript.ScriptFlags, error) {
				return common.StandardScriptVerifyFlags()
			},
		},
		ChainParams:      bm.ChainParams(),
		FetchUtxoView:    bm.GetChain().FetchUtxoView, //TODO, duplicated dependence of miner
		BlockByHash:      bm.GetChain().FetchBlockByHash,
		BestHash:         func() *hash.Hash { return &bm.GetChain().BestSnapshot().Hash },
		BestHeight:       func() uint64 { return uint64(bm.GetChain().BestSnapshot().GraphState.GetMainHeight()) },
		CalcSequenceLock: bm.GetChain().CalcSequenceLock,
		SubsidyCache:     bm.GetChain().FetchSubsidyCache(),
		SigCache:         sigCache,
		PastMedianTime:   func() time.Time { return bm.GetChain().BestSnapshot().MedianTime },
		AddrIndex:        addrIndex,
		BD:               bm.GetChain().BlockDAG(),
		BC:               bm.GetChain(),
	}
	txMemPool := mempool.New(&txC)
	invalidTx := make(map[hash.Hash]*blockdag.HashSet)
	return &TxManager{bm, txIndex, addrIndex, txMemPool, ntmgr, db, invalidTx}, nil
}
