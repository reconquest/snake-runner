package executor

import (
	"context"
	"io"
)

type ExecutorType string

const (
	EXECUTOR_DOCKER ExecutorType = "EXECUTOR_DOCKER"
	EXECUTOR_SHELL  ExecutorType = "EXECUTOR_SHELL"
)

type Executor interface {
	Type() ExecutorType
	Create(context.Context, CreateOptions) (Container, error)
	Destroy(context.Context, Container) error
	Prepare(context.Context, PrepareOptions) error
	Exec(context.Context, Container, ExecOptions) error
	DetectShell(context.Context, Container) (string, error)
	LookPath(context.Context, string) (string, error)
	Cleanup() error
}

type Container interface {
	String() string
	ID() string
}

type (
	Volume string
)

type (
	OutputConsumer func(string)
	PromptConsumer func([]string)
)

func DiscardConsumer(string) {}

type CreateOptions struct {
	Name    string
	Image   string
	Volumes []Volume
}

type ExecOptions struct {
	Cmd            []string
	Env            []string
	WorkingDir     string
	AttachStdout   bool
	AttachStderr   bool
	OutputConsumer OutputConsumer
	Stdin          io.Reader
}

type PrepareOptions struct {
	Image          string
	OutputConsumer OutputConsumer
	InfoConsumer   OutputConsumer
	Auths          []Auths
}

type Auths map[string]AuthConfig

type AuthConfig struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Auth     string `json:"auth,omitempty"`

	ServerAddress string `json:"serveraddress,omitempty"`

	// IdentityToken is used to authenticate the user and get
	// an access token for the registry.
	IdentityToken string `json:"identitytoken,omitempty"`

	// RegistryToken is a bearer token to be sent to a registry
	RegistryToken string `json:"registrytoken,omitempty"`
}

type DockerAuths struct {
	Auths Auths
}
