// Code generated by gonstructor -type CloudSidecar -constructorTypes builder; DO NOT EDIT.

package sidecar

import (
	"github.com/reconquest/snake-runner/internal/spawner"
	"github.com/reconquest/snake-runner/internal/sshkey"
)

type CloudSidecarBuilder struct {
	spawner        spawner.Spawner
	name           string
	pipelinesDir   string
	slug           string
	promptConsumer spawner.PromptConsumer
	outputConsumer spawner.OutputConsumer
	sshKey         sshkey.Key
}

func NewCloudSidecarBuilder() *CloudSidecarBuilder {
	return &CloudSidecarBuilder{}
}

func (b *CloudSidecarBuilder) Spawner(spawner spawner.Spawner) *CloudSidecarBuilder {
	b.spawner = spawner
	return b
}

func (b *CloudSidecarBuilder) Name(name string) *CloudSidecarBuilder {
	b.name = name
	return b
}

func (b *CloudSidecarBuilder) PipelinesDir(pipelinesDir string) *CloudSidecarBuilder {
	b.pipelinesDir = pipelinesDir
	return b
}

func (b *CloudSidecarBuilder) Slug(slug string) *CloudSidecarBuilder {
	b.slug = slug
	return b
}

func (b *CloudSidecarBuilder) PromptConsumer(promptConsumer spawner.PromptConsumer) *CloudSidecarBuilder {
	b.promptConsumer = promptConsumer
	return b
}

func (b *CloudSidecarBuilder) OutputConsumer(outputConsumer spawner.OutputConsumer) *CloudSidecarBuilder {
	b.outputConsumer = outputConsumer
	return b
}

func (b *CloudSidecarBuilder) SshKey(sshKey sshkey.Key) *CloudSidecarBuilder {
	b.sshKey = sshKey
	return b
}

func (b *CloudSidecarBuilder) Build() *CloudSidecar {
	return &CloudSidecar{
		spawner:        b.spawner,
		name:           b.name,
		pipelinesDir:   b.pipelinesDir,
		slug:           b.slug,
		promptConsumer: b.promptConsumer,
		outputConsumer: b.outputConsumer,
		sshKey:         b.sshKey,
	}
}
