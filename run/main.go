package run

import (
	"github.com/stroomnetwork/btcwallet/frost"
	"os"
	"runtime"
)

func ConfigureAndInitWallet() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	validators := frost.GetValidators(5, 3)
	pk1, err := validators[0].MakePubKey("test1")
	if err != nil {
		log.Info(err)
		return
	}

	pk2, err := validators[0].MakePubKey("test2")
	if err != nil {
		log.Info(err)
		return
	}

	_, err = InitWallet(validators[0], pk1, pk2)
	if err != nil {
		os.Exit(1)
	}

	<-interruptHandlersDone
	log.Info("Shutdown complete")
}
