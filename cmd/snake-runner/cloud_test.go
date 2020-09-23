package main

import (
	"context"
	"math/rand"
	"time"

	"github.com/reconquest/pkg/log"
)

var ctx = context.Background()

func init() {
	log.SetLevel(log.LevelDebug)

	rand.Seed(time.Now().UnixNano())
}

// currently unused
//
// func TestSidecar(t *testing.T) {
//    test := assert.New(t)

//    docker, err := executor.NewDocker("host")
//    test.NoError(err)

//    outputConsumer := func(text string) error {
//        log.Debugf(
//            nil,
//            "sidecar output: %s",
//            strings.TrimRight(text, "\n"),
//        )
//        return nil
//    }
//    commandConsumer := func(cmd []string) error {
//        log.Debugf(
//            nil,
//            "sidecar command: %v",
//            cmd,
//        )
//        return nil
//    }

//    sshKey := "./../../testdata/id_rsa"

//    sidecarName := utils.RandString(8)
//    dir := "/tmp/snake-runner.test.pipelines." + utils.RandString(8)

//    sidecar := sidecar.NewSidecarBuilder().
//        Cloud(docker).
//        Name(sidecarName).
//        PipelinesDir(dir).
//        Slug("testproj/testrepo").
//        OutputConsumer(outputConsumer).
//        CommandConsumer(commandConsumer).
//        SshKey(sshKey).
//        Build()

//    defer sidecar.Destroy()

//    err = sidecar.Serve(ctx, "ssh://git@localhost:7999/testproj/testrepo", "master")
//    test.NoError(err)

//    bind := sidecar.GetPipelineVolumes()[0]
//    test.True(strings.HasPrefix(bind, dir), dir)
//    test.True(strings.HasSuffix(bind, ":/pipelines/testproj/testrepo:ro"), "ro bind: %s", bind)

//    mainName := utils.RandString(8)

//    container, err := docker.CreateContainer(
//        ctx,
//        "alpine",
//        mainName,
//        sidecar.GetPipelineVolumes(),
//    )
//    if test.NoError(err) {
//        defer docker.DestroyContainer(ctx, container)

//        contents, err := docker.Cat(ctx, container, sidecar.GetContainerDir(), ".git/HEAD")
//        test.NoError(err)

//        test.Equal("ref: refs/heads/master", strings.TrimSpace(contents))
//    }
//}
