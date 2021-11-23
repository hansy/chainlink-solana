package smoke

import (
	"fmt"
	"github.com/smartcontractkit/chainlink-solana/tests/e2e/utils"
	"github.com/smartcontractkit/helmenv/environment"
)

// UploadProgramBinaries uploads programs binary files to solana-validator container
// currently it's the only way to deploy anything to local solana because ephemeral validator in k8s
// can't expose UDP ports required to copy .so chunks when deploying
func UploadProgramBinaries(e *environment.Environment) error {
	connections := e.Charts.Connections("solana-validator")
	cc, err := connections.Load("sol", "0", "sol-val")
	if err != nil {
		return err
	}
	// nolint
	_, _, _, _ = e.Charts["solana-validator"].CopyToPod(utils.ContractsDir, fmt.Sprintf("%s/%s:/programs", e.Namespace, cc.PodName), "sol-val")
	return nil
}
