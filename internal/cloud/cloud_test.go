package cloud

// func init() {
//    if !isFileExists("id_rsa_test") {
//        _, _, err := executil.Run(
//            exec.Command("ssh-keygen", "-t", "rsa", "-f", "id_rsa_test"),
//        )
//        if err != nil {
//            panic(err)
//        }
//    }
//}

// func TestMakeContainer(t *testing.T) {
//    test := assert.New(t)
//    _ = test

//    cloud, err := NewCloud()
//    if err != nil {
//        panic(err)
//    }

//    id, err := cloud.CreateContainer(
//        context.Background(),
//        "alpine",
//        "test-"+strconv.Itoa(int(time.Now().UnixNano())),
//    )
//    if err != nil {
//        panic(err)
//    }

//    err = cloud.PrepareContainer(id, "id_rsa_test", func(text string) error {
//        log.Println(strings.TrimSpace(text))
//        return nil
//    })
//    if err != nil {
//        panic(err)
//    }
//}
