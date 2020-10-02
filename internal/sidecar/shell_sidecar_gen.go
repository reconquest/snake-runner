// Code generated by gonstructor -type ShellSidecar -constructorTypes builder; DO NOT EDIT.

package sidecar

import (
	"github.com/reconquest/snake-runner/internal/executor"
	"github.com/reconquest/snake-runner/internal/sshkey"
)

type ShellSidecarBuilder struct {
	executor       executor.Executor
	name           string
	slug           string
	promptConsumer executor.PromptConsumer
	outputConsumer executor.OutputConsumer
	pipelinesDir   string
	sshKey         sshkey.Key
}

func NewShellSidecarBuilder() *ShellSidecarBuilder {
	return &ShellSidecarBuilder{}
}

func (b *ShellSidecarBuilder) Executor(executor executor.Executor) *ShellSidecarBuilder {
	b.executor = executor
	return b
}

func (b *ShellSidecarBuilder) Name(name string) *ShellSidecarBuilder {
	b.name = name
	return b
}

func (b *ShellSidecarBuilder) Slug(slug string) *ShellSidecarBuilder {
	b.slug = slug
	return b
}

func (b *ShellSidecarBuilder) PromptConsumer(promptConsumer executor.PromptConsumer) *ShellSidecarBuilder {
	b.promptConsumer = promptConsumer
	return b
}

func (b *ShellSidecarBuilder) OutputConsumer(outputConsumer executor.OutputConsumer) *ShellSidecarBuilder {
	b.outputConsumer = outputConsumer
	return b
}

func (b *ShellSidecarBuilder) PipelinesDir(pipelinesDir string) *ShellSidecarBuilder {
	b.pipelinesDir = pipelinesDir
	return b
}

func (b *ShellSidecarBuilder) SshKey(sshKey sshkey.Key) *ShellSidecarBuilder {
	b.sshKey = sshKey
	return b
}

func (b *ShellSidecarBuilder) Build() *ShellSidecar {
	return &ShellSidecar{
		executor:       b.executor,
		name:           b.name,
		slug:           b.slug,
		promptConsumer: b.promptConsumer,
		outputConsumer: b.outputConsumer,
		pipelinesDir:   b.pipelinesDir,
		sshKey:         b.sshKey,
	}
}