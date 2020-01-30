package cloud

type logwriter struct {
	callback OutputConsumer
}

func (logwriter logwriter) Write(data []byte) (int, error) {
	if logwriter.callback == nil {
		return len(data), nil
	}
	logwriter.callback(string(data))
	return len(data), nil
}
