package common

import (
	"github.com/btceasypay/bitcoinpay/engine/txscript"
	"github.com/btceasypay/bitcoinpay/services/mempool"
)

// standardScriptVerifyFlags returns the script flags that should be used when
// executing transaction scripts to enforce additional checks which are required
// for the script to be considered standard.  Note these flags are different
// than what is required for the consensus rules in that they are more strict.
func StandardScriptVerifyFlags() (txscript.ScriptFlags, error) {
	scriptFlags := mempool.BaseStandardVerifyFlags
	return scriptFlags, nil
}
