package main

type logwriter struct {
	callback func(string) error
}

func (logwriter logwriter) Write(data []byte) (int, error) {
	err := logwriter.callback(string(data))
	if err != nil {
		return 0, err
	}

	return len(data), nil
}
