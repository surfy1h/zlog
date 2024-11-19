package zlog

import (
	"fmt"
	"net/http"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func InitLogServer(port int32) {
	go func() {
		http.HandleFunc("/updateLevel", func(w http.ResponseWriter, r *http.Request) {
			// Parse query parameters
			queryParams := r.URL.Query()
			// Get a single parameter
			level := queryParams.Get("level")
			l, err := zapcore.ParseLevel(level)
			if err != nil {
				w.Write([]byte(`{"code":400,"message":"parse level failed"}`))
				return
			}

			UpdateLoggerLevel(l)
		})

		if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), nil); err != nil {
			zap.S().Error("init log server start failed", zap.Error(err))
		}
	}()
}
