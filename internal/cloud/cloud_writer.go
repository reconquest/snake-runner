package cloud

import (
	"context"

	"github.com/reconquest/snake-runner/internal/utils"
)

type callbackWriter struct {
	ctx      context.Context
	callback OutputConsumer
}

func (callbackWriter callbackWriter) Write(data []byte) (int, error) {
	if callbackWriter.callback == nil {
		return len(data), nil
	}

	if utils.IsDone(callbackWriter.ctx) {
		return 0, context.Canceled
	}

	callbackWriter.callback(string(data))

	return len(data), nil
}
