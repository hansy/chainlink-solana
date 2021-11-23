package solclient

import (
	utils2 "github.com/smartcontractkit/chainlink-solana/tests/e2e/utils"
	"github.com/smartcontractkit/helmenv/environment"
	"path/filepath"
)

// NewChainlinkSolConfig returns a cluster config with Solana test validator
func NewChainlinkSolConfig(chainlinkValues map[string]interface{}) *environment.Config {
	return &environment.Config{
		NamespacePrefix: "chainlink-sol",
		Charts: environment.Charts{
			"solana-validator": {
				Index: 1,
				Path:  filepath.Join(utils2.ChartsRoot, "solana-validator"),
			},
			//"chainlink": environment.NewChainlinkChart(4, chainlinkValues),
		},
	}
}
