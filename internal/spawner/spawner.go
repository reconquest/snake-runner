package spawner

import (
	"context"
)

type Spawner interface {
	Create(context.Context, Name, Image, []Volume) (Container, error)
	Destroy(context.Context, Container) error
	Prepare(ctx context.Context, image Image, output, info OutputConsumer, pullConfig []PullConfig) error
	Exec(context.Context, Container, ExecConfig, OutputConsumer) error
	Cleanup() error
}

type Container interface {
	String() string
	ID() string
}

type (
	Name   string
	Image  string
	Volume string
)

type (
	OutputConsumer func(string)
	PromptConsumer func([]string)
)

func DiscardConsumer(string) {}

type ExecConfig struct {
	Cmd          []string
	Env          []string
	WorkingDir   string
	AttachStdout bool
	AttachStderr bool
}

type PullConfig struct {
	Auths map[string]AuthConfig `json:"auths"`
}

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
