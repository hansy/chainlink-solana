package utils

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"github.com/smartcontractkit/integrations-framework/contracts"
	"github.com/smartcontractkit/libocr/offchainreporting2/confighelper"
	"github.com/smartcontractkit/libocr/offchainreporting2/types"
	"golang.org/x/crypto/curve25519"
)

func chunkSlice(items []byte, chunkSize int) (chunks [][]byte) {
	for chunkSize < len(items) {
		chunks = append(chunks, items[0:chunkSize])
		items = items[chunkSize:]
	}
	return append(chunks, items)
}

func oracleIdentitiesFrom() ([]confighelper.OracleIdentityExtra, error) {
	oracle := solana.NewWallet()

	oracleOffChainPubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	configPubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	cfgPubKeyBytes := [curve25519.PointSize]byte{}
	copy(cfgPubKeyBytes[:], configPubKey)
	oracleIdentities := []confighelper.OracleIdentityExtra{}
	for i := 0; i < 20; i++ {
		oracleIdentities = append(oracleIdentities, confighelper.OracleIdentityExtra{
			OracleIdentity: confighelper.OracleIdentity{
				OffchainPublicKey: []byte(oracleOffChainPubKey),
				OnchainPublicKey:  oracle.PublicKey().Bytes(),
				PeerID:            fmt.Sprintf("oracle_%d", i),
				TransmitAccount:   types.Account(oracle.PublicKey().String()),
			},
			ConfigEncryptionPublicKey: cfgPubKeyBytes,
		})
	}
	return oracleIdentities, nil
}

func NewOCR2Config(cfg contracts.OffChainAggregatorV2Config) (
	version uint64,
	offchainConfigChunks [][]byte,
	err error,
) {
	oi, err := oracleIdentitiesFrom()
	if err != nil {
		return 0, nil, err
	}
	_, _, _, _, version, cfgBytes, err := confighelper.ContractSetConfigArgs(
		cfg.DeltaProgress,
		cfg.DeltaResend,
		cfg.DeltaRound,
		cfg.DeltaGrace,
		cfg.DeltaStage,
		cfg.RMax,
		cfg.S,
		oi,
		[]byte{},
		cfg.MaxDurationQuery,
		cfg.MaxDurationObservation,
		cfg.MaxDurationReport,
		cfg.MaxDurationShouldAcceptFinalizedReport,
		cfg.MaxDurationShouldTransmitAcceptedReport,
		cfg.F,
		cfg.OnchainConfig,
	)
	return version, chunkSlice(cfgBytes, 800), nil
}
