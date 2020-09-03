package spawner

import (
	"context"
	"io"
)

type SpawnerType string

const (
	SPAWNER_DOCKER SpawnerType = "SPAWNER_DOCKER"
	SPAWNER_SHELL  SpawnerType = "SPAWNER_SHELL"
)

type Spawner interface {
	Type() SpawnerType
	Create(context.Context, CreateOptions) (Container, error)
	Destroy(context.Context, Container) error
	Prepare(context.Context, PrepareOptions) error
	Exec(context.Context, Container, ExecOptions) error
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
