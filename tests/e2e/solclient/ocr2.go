package solclient

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/chainlink-solana/tests/e2e/generated/ocr_2"
	"github.com/smartcontractkit/chainlink-solana/tests/e2e/utils"
	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/contracts"
)

type OCRv2 struct {
	Client        *Client
	State         *solana.Wallet
	Authorities   map[string]*Authority
	Payees        []*solana.Wallet
	ProgramWallet *solana.Wallet
}

func (m *OCRv2) writeOffChainConfig(ocConfigBytes []byte) error {
	payer := m.Client.DefaultWallet
	err := m.Client.TXSync(
		"Write OffChain config chunk",
		rpc.CommitmentFinalized,
		[]solana.Instruction{
			ocr_2.NewWriteOffchainConfigInstruction(
				ocConfigBytes,
				m.State.PublicKey(),
				m.Client.Owner.PublicKey(),
			).Build(),
		},
		func(key solana.PublicKey) *solana.PrivateKey {
			if key.Equals(m.Client.Owner.PublicKey()) {
				return &m.Client.Owner.PrivateKey
			}
			if key.Equals(payer.PublicKey()) {
				return &payer.PrivateKey
			}
			return nil
		},
		payer.PublicKey(),
	)
	if err != nil {
		return err
	}
	return nil
}

func (m *OCRv2) commitOffChainConfig() error {
	payer := m.Client.DefaultWallet
	err := m.Client.TXSync(
		"Commit OffChain config",
		rpc.CommitmentFinalized,
		[]solana.Instruction{
			ocr_2.NewCommitOffchainConfigInstruction(
				m.State.PublicKey(),
				m.Client.Owner.PublicKey(),
			).Build(),
		},
		func(key solana.PublicKey) *solana.PrivateKey {
			if key.Equals(m.Client.Owner.PublicKey()) {
				return &m.Client.Owner.PrivateKey
			}
			if key.Equals(payer.PublicKey()) {
				return &payer.PrivateKey
			}
			return nil
		},
		payer.PublicKey(),
	)
	if err != nil {
		return err
	}
	return nil
}

func (m *OCRv2) beginOffChainConfig(version uint64) error {
	payer := m.Client.DefaultWallet
	err := m.Client.TXSync(
		"Begin OffChain config",
		rpc.CommitmentFinalized,
		[]solana.Instruction{
			ocr_2.NewBeginOffchainConfigInstruction(
				version,
				m.State.PublicKey(),
				m.Client.Owner.PublicKey(),
			).Build(),
		},
		func(key solana.PublicKey) *solana.PrivateKey {
			if key.Equals(m.Client.Owner.PublicKey()) {
				return &m.Client.Owner.PrivateKey
			}
			if key.Equals(payer.PublicKey()) {
				return &payer.PrivateKey
			}
			return nil
		},
		payer.PublicKey(),
	)
	if err != nil {
		return err
	}
	return nil
}

// SetOffChainConfig sets offchain config in multiple transactions
func (m *OCRv2) SetOffChainConfig(ocParams contracts.OffChainAggregatorV2Config) error {
	_, cfgChunks, err := utils.NewOCR2Config(ocParams)
	if err != nil {
		return err
	}
	if err := m.beginOffChainConfig(1); err != nil {
		return err
	}
	for _, cfgChunk := range cfgChunks {
		if err := m.writeOffChainConfig(cfgChunk); err != nil {
			return err
		}
	}
	if err := m.commitOffChainConfig(); err != nil {
		return err
	}
	return nil
}

// DumpState dumps all OCR accounts state
func (m *OCRv2) DumpState() error {
	var stateDump *ocr_2.State
	err := m.Client.RPC.GetAccountDataInto(
		context.Background(),
		m.Client.OCRStateAcc.PublicKey(),
		&stateDump,
	)
	if err != nil {
		return err
	}
	log.Warn().Interface("State", stateDump).Send()
	return nil
}

// SetValidatorConfig sets validator config
func (m *OCRv2) SetValidatorConfig(flaggingThreshold uint32, validatorAddr string) error {
	payer := m.Client.DefaultWallet
	validatorPubKey, err := solana.PublicKeyFromBase58(validatorAddr)
	if err != nil {
		return err
	}
	err = m.Client.TXAsync(
		"Set validator config",
		[]solana.Instruction{
			ocr_2.NewSetValidatorConfigInstruction(
				flaggingThreshold,
				m.Client.OCRStateAcc.PublicKey(),
				m.Client.Owner.PublicKey(),
				validatorPubKey,
			).Build(),
		},
		func(key solana.PublicKey) *solana.PrivateKey {
			if key.Equals(m.Client.Owner.PublicKey()) {
				return &m.Client.Owner.PrivateKey
			}
			if key.Equals(payer.PublicKey()) {
				return &payer.PrivateKey
			}
			return nil
		},
		payer.PublicKey(),
	)
	if err != nil {
		return err
	}
	return nil
}

// SetBilling sets default billing to oracles
func (m *OCRv2) SetBilling(price uint32, controllerAddr string) error {
	payer := m.Client.DefaultWallet
	billingACPubKey, err := solana.PublicKeyFromBase58(controllerAddr)
	if err != nil {
		return nil
	}
	err = m.Client.TXAsync(
		"Set billing",
		[]solana.Instruction{
			ocr_2.NewSetBillingInstruction(
				price,
				m.Client.OCRStateAcc.PublicKey(),
				m.Client.Owner.PublicKey(),
				billingACPubKey,
			).Build(),
		},
		func(key solana.PublicKey) *solana.PrivateKey {
			if key.Equals(m.Client.Owner.PublicKey()) {
				return &m.Client.Owner.PrivateKey
			}
			if key.Equals(payer.PublicKey()) {
				return &payer.PrivateKey
			}
			return nil
		},
		payer.PublicKey(),
	)
	if err != nil {
		return err
	}
	return nil
}

func (m *OCRv2) GetContractData(ctx context.Context) (*contracts.OffchainAggregatorData, error) {
	panic("implement me")
}

// SetOracles sets oracles with payee addresses
func (m *OCRv2) SetOracles(chainlinkNodes []client.Chainlink, f int) error {
	log.Info().Str("Program Address", m.ProgramWallet.PublicKey().String()).Msg("Setting oracles")
	payer := m.Client.DefaultWallet
	instr := make([]solana.Instruction, 0)

	oracles := make([]ocr_2.NewOracle, 0)
	transmitterPubKeys := make([]string, 0)

	// get keys from nodes
	for i := 0; i < 5; i++ {
		// get report signing key somehow?
		w := solana.NewWallet()
		key := make([]byte, 20)
		var keyArr [20]byte
		if _, err := rand.Read(key); err != nil {
			return err
		}
		copy(keyArr[:], key)
		oracles = append(oracles, ocr_2.NewOracle{
			Signer:      keyArr,
			Transmitter: w.PublicKey(),
		})
		transmitterPubKeys = append(transmitterPubKeys, w.PublicKey().String())
	}
	// set one payee for all
	payee := solana.NewWallet()
	if err := m.Client.addNewAssociatedAccInstr(payee, m.Client.Owner.PublicKey(), &instr); err != nil {
		return err
	}
	payees := make([]solana.PublicKey, 0)
	for i := 0; i < len(transmitterPubKeys); i++ {
		payees = append(payees, payee.PublicKey())
	}
	instr = append(instr, ocr_2.NewSetConfigInstruction(
		oracles,
		uint8(f),
		m.Client.OCRStateAcc.PublicKey(),
		m.Client.Owner.PublicKey(),
	).Build())
	instr = append(instr, ocr_2.NewSetPayeesInstruction(
		payees,
		m.State.PublicKey(),
		m.Client.Owner.PublicKey()).Build(),
	)
	err := m.Client.TXAsync(
		"Set oracles with associated payees",
		instr,
		func(key solana.PublicKey) *solana.PrivateKey {
			if key.Equals(payee.PublicKey()) {
				return &payee.PrivateKey
			}
			if key.Equals(m.Client.Owner.PublicKey()) {
				return &m.Client.Owner.PrivateKey
			}
			if key.Equals(payer.PublicKey()) {
				return &payer.PrivateKey
			}
			return nil
		},
		payer.PublicKey(),
	)
	if err != nil {
		return err
	}
	return nil
}

func (m *OCRv2) RequestNewRound() error {
	panic("implement me")
}

func (m *OCRv2) AuthorityAddr(s string) (string, error) {
	auth, ok := m.Authorities[s]
	if !ok {
		return "", fmt.Errorf("authority with seed %s not found", s)
	}
	return auth.AuthorityPubKey.String(), nil
}

func (m *OCRv2) Address() string {
	return m.State.PublicKey().String()
}

func (m *OCRv2) TransferOwnership(to string) error {
	panic("implement me")
}

func (m *OCRv2) GetLatestConfigDetails() (map[string]interface{}, error) {
	panic("implement me")
}

func (m *OCRv2) GetRoundData(roundID uint32) (map[string]interface{}, error) {
	panic("implement me")
}

func (m *OCRv2) GetOwedPayment(transmitterAddr string) (map[string]interface{}, error) {
	panic("implement me")
}
