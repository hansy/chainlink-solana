package solclient

import "github.com/gagliardetto/solana-go"

type DeviationFlaggingValidator struct {
	Client        *Client
	State         *solana.Wallet
	ProgramWallet *solana.Wallet
}

func (d *DeviationFlaggingValidator) Address() string {
	return d.State.PublicKey().String()
}
