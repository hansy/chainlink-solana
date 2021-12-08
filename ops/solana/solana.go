package solana

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	relayUtils "github.com/smartcontractkit/chainlink-relay/ops/utils"
)

// Contracts
const (
	AccessController = iota
	OCR2
)

// Contract States
const (
	BillingAccessController = iota
	RequesterAccessController
	OCRFeed
	LINK
)

type Deployer struct {
	gauntlet  relayUtils.Gauntlet
	network   string
	Contracts map[int]string
	States    map[int]string
}

func New(ctx *pulumi.Context) (Deployer, error) {

	// yarn, err := exec.LookPath("yarn")
	// if err != nil {
	// 	return Deployer{}, errors.New("'yarn' is not installed")
	// }
	// fmt.Printf("yarn is available at %s\n", yarn)
	//
	// // Change path to root directory
	cwd, _ := os.Getwd()
	// os.Chdir(filepath.Join(cwd, ".."))
	//
	// fmt.Println("Installing dependencies")
	// if _, err = exec.Command(yarn).Output(); err != nil {
	// 	return Deployer{}, errors.New("error install dependencies")
	// }
	//
	// // Generate Gauntlet Binary
	// fmt.Println("Generating Gauntlet binary...")
	// _, err = exec.Command(yarn, "bundle").Output()
	// if err != nil {
	// 	return Deployer{}, errors.New("error generating gauntlet binary")
	// }

	// TODO: Should come from pulumi context
	os.Setenv("SKIP_PROMPTS", "true")

	version := "linux"
	if config.Get(ctx, "VERSION") == "MACOS" {
		version = "macos"
	}

	// Check gauntlet works
	os.Chdir(cwd) // move back into ops folder
	gauntletBin := filepath.Join(cwd, "../bin/chainlink-solana-") + version
	gauntlet, err := relayUtils.NewGauntlet(gauntletBin)

	if err != nil {
		return Deployer{}, err
	}

	return Deployer{
		gauntlet:  gauntlet,
		network:   "local",
		Contracts: make(map[int]string),
		States:    make(map[int]string),
	}, nil
}

func (d *Deployer) Load() error {
	// TODO: remove this - temporarily needed as artifacts are read directly from the root directory
	// won't be needed once it reads from release artifacts?
	cwd, _ := os.Getwd()
	os.Chdir(filepath.Join(cwd, "..")) // go from ops folder to root

	// Access Controller contract deployment
	fmt.Println("Deploying Access Controller...")
	err := d.gauntlet.ExecCommand(
		"access_controller:deploy",
		d.gauntlet.Flag("network", d.network),
	)
	if err != nil {
		return errors.New("access controller contract deployment failed")
	}

	report, err := d.gauntlet.ReadCommandReport()
	if err != nil {
		return errors.New("report not available")
	}

	d.Contracts[AccessController] = report.Responses[0].Contract

	// OCR2 contract deployment
	fmt.Println("Deploying OCR 2...")
	err = d.gauntlet.ExecCommand(
		"ocr2:deploy",
		d.gauntlet.Flag("network", d.network),
	)
	if err != nil {
		return errors.New("ocr 2 contract deployment failed")
	}

	report, err = d.gauntlet.ReadCommandReport()
	if err != nil {
		return errors.New("report not available")
	}
	d.Contracts[OCR2] = report.Responses[0].Contract

	return nil
}

func (d *Deployer) DeployLINK() error {
	fmt.Println("Deploying LINK Token...")
	err := d.gauntlet.ExecCommand(
		"token:deploy",
		d.gauntlet.Flag("network", d.network),
	)
	if err != nil {
		return errors.New("LINK contract deployment failed")
	}

	report, err := d.gauntlet.ReadCommandReport()
	if err != nil {
		return errors.New("report not available")
	}

	linkAddress := report.Responses[0].Contract
	d.States[LINK] = linkAddress

	return nil
}

func (d *Deployer) DeployOCR() error {
	fmt.Println("Initializing OCR Feed...")
	err := d.gauntlet.ExecCommand(
		"ocr2:initialize:flow",
		d.gauntlet.Flag("network", d.network),
		d.gauntlet.Flag("description", "ETH/USD"),
		d.gauntlet.Flag("decimals", "8"),
		d.gauntlet.Flag("maxAnswer", "100000000000000000000"),
		d.gauntlet.Flag("minAnswer", "0"),
		d.gauntlet.Flag("link", d.States[LINK]),
	)
	if err != nil {
		return errors.New("feed initialization failed")
	}

	report, err := d.gauntlet.ReadCommandFlowReport()
	if err != nil {
		return err
	}

	d.States[BillingAccessController] = report[0].Txs[0].Contract
	d.States[RequesterAccessController] = report[1].Txs[0].Contract
	d.States[OCRFeed] = report[2].Txs[0].Contract

	return nil
}

func (d Deployer) TransferLINK() error {
	err := d.gauntlet.ExecCommand(
		"token:transfer",
		d.gauntlet.Flag("network", d.network),
		d.gauntlet.Flag("to", d.States[OCRFeed]),
		d.gauntlet.Flag("amount", "10000"),
		d.States[LINK],
	)
	if err != nil {
		return errors.New("LINK transfer failed")
	}

	return nil
}

func (d Deployer) InitOCR(keys []map[string]string) error {

	jsonKeys, err := json.Marshal(keys)
	if err != nil {
		return err
	}

	err = d.gauntlet.ExecCommand(
		"ocr2:set_config:deployer",
		d.gauntlet.Flag("network", d.network),
		d.gauntlet.Flag("keys", string(jsonKeys)),
		d.gauntlet.Flag("state", d.States[OCRFeed]),
	)

	if err != nil {
		return errors.New("OCR 2 set config failed")
	}
	return nil
}

func (d Deployer) Fund(addresses []string) error {
	if _, err := exec.LookPath("solana"); err != nil {
		return errors.New("'solana' is not available in commandline")
	}
	for _, a := range addresses {
		// TODO: do addresses need to parsed into base58 form?
		msg := relayUtils.LogStatus(fmt.Sprintf("funded %s", a))
		if _, err := exec.Command("solana", "airdrop", "100", a).Output(); msg.Check(err) != nil {
			return err
		}
	}
	return nil
}

func (d Deployer) OCR2Address() string {
	return d.States[OCRFeed]
}

func (d Deployer) Addresses() map[int]string {
	return d.States
}
