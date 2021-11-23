package deployercli

import (
	"bytes"
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/chainlink-solana/tests/e2e/utils"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type CLIConfig struct {
	JSONRPCURL    string `yaml:"json_rpc_url"`
	WebsocketURL  string `yaml:"websocket_url"`
	KeypairPath   string `yaml:"keypair_path"`
	AddressLabels struct {
		Num11111111111111111111111111111111 string `yaml:"11111111111111111111111111111111"`
	} `yaml:"address_labels"`
	Commitment string `yaml:"commitment"`
}

type ExecResult struct {
	StdOut   string
	StdErr   string
	ExitCode int
}

func (ss *DockerShell) exec(ctx context.Context, containerID string, command []string) (types.IDResponse, error) {
	config := types.ExecConfig{
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          command,
	}
	return ss.cli.ContainerExecCreate(ctx, containerID, config)
}

func (ss *DockerShell) inspectResult(ctx context.Context, id string) (ExecResult, error) {
	var execResult ExecResult
	resp, err := ss.cli.ContainerExecAttach(ctx, id, types.ExecStartCheck{})
	if err != nil {
		return execResult, err
	}
	defer resp.Close()
	// read the output
	var outBuf, errBuf bytes.Buffer
	outputDone := make(chan error)
	go func() {
		// StdCopy demultiplexes the stream into two buffers
		_, err = stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader)
		outputDone <- err
	}()
	select {
	case err := <-outputDone:
		if err != nil {
			return execResult, err
		}
		break
	case <-ctx.Done():
		return execResult, ctx.Err()
	}

	stdout, err := ioutil.ReadAll(&outBuf)
	if err != nil {
		return execResult, err
	}
	stderr, err := ioutil.ReadAll(&errBuf)
	if err != nil {
		return execResult, err
	}

	res, err := ss.cli.ContainerExecInspect(ctx, id)
	if err != nil {
		return execResult, err
	}

	execResult.ExitCode = res.ExitCode
	execResult.StdOut = string(stdout)
	execResult.StdErr = string(stderr)
	return execResult, nil
}

type DockerShell struct {
	containerID string
	cli         *client.Client
}

func NewDockerShell(c *client.Client) *DockerShell {
	return &DockerShell{
		cli: c,
	}
}

func (ss *DockerShell) Start() error {
	ctx := context.Background()
	cliCfg := &CLIConfig{
		JSONRPCURL:   "http://host.docker.internal:8899",
		WebsocketURL: "ws://host.docker.internal:8900",
		KeypairPath:  "keys/id.json",
		AddressLabels: struct {
			Num11111111111111111111111111111111 string `yaml:"11111111111111111111111111111111"`
		}{},
		Commitment: "finalized",
	}
	cfgBytes, err := yaml.Marshal(cliCfg)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(utils.KeysDir, "config.yml"), cfgBytes, os.ModePerm); err != nil {
		return err
	}
	mounts := []mount.Mount{
		{
			Type:     "bind",
			Source:   utils.ContractsDir,
			Target:   "/contracts",
			ReadOnly: false,
		},
		{
			Type:     "bind",
			Source:   utils.KeysDir,
			Target:   "/keys",
			ReadOnly: false,
		},
	}
	resp, err := ss.cli.ContainerCreate(ctx, &container.Config{Image: "sol-cli"}, &container.HostConfig{Mounts: mounts, NetworkMode: "host"}, nil, nil, "sol-cli")
	if err != nil {
		return err
	}
	ss.containerID = resp.ID
	if err := ss.cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}
	return nil
}

func (ss *DockerShell) RunCMD(cmd string) error {
	cm := strings.Split(cmd, " ")
	rID, err := ss.exec(context.Background(), ss.containerID, cm)
	if err != nil {
		panic(err)
	}
	result, err := ss.inspectResult(context.Background(), rID.ID)
	if err != nil {
		panic(err)
	}
	log.Warn().
		Str("CMD", cmd).
		Str("StdOut", result.StdOut).
		Str("StdErr", result.StdErr).
		Msg("CLI Response")
	return nil
}

func (ss *DockerShell) Stop() error {
	if err := ss.cli.ContainerKill(context.Background(), ss.containerID, "KILL"); err != nil {
		return err
	}
	if err := ss.cli.ContainerRemove(context.Background(), ss.containerID, types.ContainerRemoveOptions{}); err != nil {
		return err
	}
	return nil
}
