package smoke

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	solclient2 "github.com/smartcontractkit/chainlink-solana/tests/e2e/solclient"
	utils2 "github.com/smartcontractkit/chainlink-solana/tests/e2e/utils"
	"github.com/smartcontractkit/helmenv/environment"
	"github.com/smartcontractkit/integrations-framework/actions"
	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/contracts"
)

var _ = Describe("Solana OCRv2", func() {

	var (
		e              *environment.Environment
		cd             contracts.ContractDeployer
		chainlinkNodes []client.Chainlink
		validator      contracts.DeviationFlaggingValidator
		billingAC      contracts.OCRv2AccessController
		requesterAC    contracts.OCRv2AccessController
		ocr2           contracts.OCRv2
		nets           *client.Networks
		err            error
	)

	BeforeEach(func() {
		By("Deploying the environment", func() {
			e, err = environment.DeployOrLoadEnvironment(
				solclient2.NewChainlinkSolConfig(nil),
				utils2.ChartsRoot,
			)
			Expect(err).ShouldNot(HaveOccurred())
			err = e.ConnectAll()
			Expect(err).ShouldNot(HaveOccurred())
			err = UploadProgramBinaries(e)
			Expect(err).ShouldNot(HaveOccurred())
		})
		By("Getting the clients", func() {
			networkRegistry := client.NewNetworkRegistry()
			networkRegistry.RegisterNetwork(
				"solana",
				solclient2.ClientInitFunc(),
				solclient2.ClientURLSFunc(),
			)
			nets, err = networkRegistry.GetNetworks(e)
			Expect(err).ShouldNot(HaveOccurred())
			//chainlinkNodes, err = client.NewChainlinkClients(e)
			//Expect(err).ShouldNot(HaveOccurred())
		})
		By("Deploying contracts", func() {
			cd, err = solclient2.NewContractDeployer(nets.Default, e)
			Expect(err).ShouldNot(HaveOccurred())
			lt, err := cd.DeployLinkTokenContract()
			Expect(err).ShouldNot(HaveOccurred())
			billingAC, err = cd.DeployOCRv2AccessController()
			Expect(err).ShouldNot(HaveOccurred())
			requesterAC, err = cd.DeployOCRv2AccessController()
			Expect(err).ShouldNot(HaveOccurred())
			err = nets.Default.WaitForEvents()
			Expect(err).ShouldNot(HaveOccurred())

			validator, err = cd.DeployOCRv2DeviationFlaggingValidator(billingAC.Address())
			Expect(err).ShouldNot(HaveOccurred())
			ocr2, err = cd.DeployOCRv2(billingAC.Address(), requesterAC.Address(), lt.Address())
			Expect(err).ShouldNot(HaveOccurred())
			err = nets.Default.WaitForEvents()
			Expect(err).ShouldNot(HaveOccurred())

			err = ocr2.SetValidatorConfig(uint32(80000), validator.Address())
			Expect(err).ShouldNot(HaveOccurred())
			err = ocr2.SetBilling(uint32(1), billingAC.Address())
			Expect(err).ShouldNot(HaveOccurred())
			validatorAuth, err := ocr2.AuthorityAddr("validator")
			Expect(err).ShouldNot(HaveOccurred())
			err = billingAC.AddAccess(validatorAuth)
			Expect(err).ShouldNot(HaveOccurred())
			chainlinkNodes = []client.Chainlink{}
			err = ocr2.SetOracles(chainlinkNodes, 1)
			Expect(err).ShouldNot(HaveOccurred())

			err = nets.Default.WaitForEvents()
			Expect(err).ShouldNot(HaveOccurred())

			//err = ocr2.SetOffChainConfig(contracts.OffChainAggregatorV2Config{
			//	DeltaProgress:                           2 * time.Second,
			//	DeltaResend:                             1 * time.Second,
			//	DeltaRound:                              1 * time.Second,
			//	DeltaGrace:                              500 * time.Millisecond,
			//	DeltaStage:                              2*time.Second,
			//	RMax:                                    3,
			//	S:                                       nil,
			//	Oracles:                                 nil,
			//	ReportingPluginConfig:                   nil,
			//	MaxDurationQuery:                        0,
			//	MaxDurationObservation:                  0,
			//	MaxDurationReport:                       0,
			//	MaxDurationShouldAcceptFinalizedReport:  0,
			//	MaxDurationShouldTransmitAcceptedReport: 0,
			//	F:                                       0,
			//	OnchainConfig:                           nil,
			//})
			//Expect(err).ShouldNot(HaveOccurred())
			//err = ocr2.DumpState()
			//Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Describe("with Solana", func() {
		It("performs OCR round", func() {
		})
	})

	AfterEach(func() {
		By("Tearing down the environment", func() {
			err = actions.TeardownSuite(e, nil, "logs")
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})
