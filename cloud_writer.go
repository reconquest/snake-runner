package main

type logwriter struct {
	callback func(string) error
}

func (logwriter logwriter) Write(data []byte) (int, error) {
	if logwriter.callback == nil {
		return len(data), nil
	}
	err := logwriter.callback(string(data))
	if err != nil {
		return 0, err
	}

	return len(data), nil
}